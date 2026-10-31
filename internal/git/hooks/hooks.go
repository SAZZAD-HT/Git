package hooks

import (
	"bufio"
	"bytes"
	"context"
	"crypto/subtle"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"anvil/internal/explorer"
	"anvil/internal/fsutil"
	"anvil/internal/logging"
	"anvil/internal/repos"
)

// Environment variables the smart HTTP handler passes to git receive-pack, which passes
// them on to the hooks (docs/reference/04 §3.1).
const (
	EnvRepoID        = "ANVIL_REPO_ID"
	EnvPusherID      = "ANVIL_PUSHER_ID"
	EnvInternalURL   = "ANVIL_INTERNAL_URL"
	EnvInternalToken = "ANVIL_INTERNAL_TOKEN"
)

// Names of the hooks Anvil installs.
var Names = []string{"pre-receive", "post-receive"}

// Installer writes hook scripts that call AnvilBinary. It implements repos.HookInstaller.
type Installer struct {
	AnvilBinary string // absolute path of the running anvil executable
}

// Install writes Anvil's pre-receive and post-receive scripts into repoPath/hooks, pointing
// at the anvil binary. Existing Anvil hooks are overwritten; git's sample hooks are removed.
func (in Installer) Install(repoPath string) error {
	dir := filepath.Join(repoPath, "hooks")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	if samples, _ := filepath.Glob(filepath.Join(dir, "*.sample")); len(samples) > 0 {
		for _, s := range samples {
			_ = os.Remove(s)
		}
	}
	for _, name := range Names {
		if err := fsutil.WriteFileAtomic(filepath.Join(dir, name), []byte(hookScript(in.AnvilBinary, name)), 0o755); err != nil {
			return err
		}
	}
	return nil
}

// hookScript returns the sh script for hook name that execs the anvil binary at bin.
func hookScript(bin, name string) string {
	// Forward slashes: Git for Windows' sh.exe understands C:/path/anvil.exe.
	return fmt.Sprintf("#!/bin/sh\n# Installed by Anvil — do not edit. Reinstalled on upgrade.\nexec %s hook %s\n",
		shQuote(filepath.ToSlash(bin)), name)
}

// shQuote quotes s as a single POSIX sh word. Single quotes stop sh from expanding $, `
// and \ (all legal in Unix paths); each embedded single quote closes the quoting, is
// written escaped, and reopens it.
func shQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

// request is the body sent to the internal API.
type request struct {
	RepoID   int64             `json:"repo_id"`
	PusherID int64             `json:"pusher_id"`
	Updates  []repos.RefUpdate `json:"updates"`
	// Env carries the quarantine object directories pre-receive needs to see pushed objects.
	Env []string `json:"env,omitempty"`
}

// response is the internal API's answer.
type response struct {
	Allowed  bool     `json:"allowed"`
	Message  string   `json:"message,omitempty"`
	Messages []string `json:"messages,omitempty"`
}

// RunClient is the body of `anvil hook <name>`: it reads stdin, calls the internal API
// using ANVIL_INTERNAL_URL/TOKEN from the environment, prints messages to stderr,
// and returns the process exit code (0 allows the push).
//
// If the variables are missing, the push did not come through Anvil's HTTP endpoint (an
// administrator pushing on disk); the push is allowed (docs/reference/04 §3.1). If the
// server cannot be reached, pre-receive rejects the push (fail closed) and post-receive
// only warns.
func RunClient(ctx context.Context, name string, stdin io.Reader, stderr io.Writer, env []string) int {
	vars := map[string]string{}
	var quarantine []string
	for _, kv := range env {
		k, v, _ := strings.Cut(kv, "=")
		vars[k] = v
		if k == "GIT_OBJECT_DIRECTORY" || k == "GIT_ALTERNATE_OBJECT_DIRECTORIES" || k == "GIT_QUARANTINE_PATH" {
			quarantine = append(quarantine, kv)
		}
	}
	base, token := vars[EnvInternalURL], vars[EnvInternalToken]
	if base == "" || token == "" {
		return 0
	}
	req := request{Env: quarantine}
	req.RepoID, _ = strconv.ParseInt(vars[EnvRepoID], 10, 64)
	req.PusherID, _ = strconv.ParseInt(vars[EnvPusherID], 10, 64)
	sc := bufio.NewScanner(stdin)
	for sc.Scan() {
		f := strings.Fields(sc.Text())
		if len(f) == 3 {
			req.Updates = append(req.Updates, repos.RefUpdate{Old: f[0], New: f[1], Ref: f[2]})
		}
	}
	fail := func(format string, a ...any) int {
		fmt.Fprintf(stderr, format+"\n", a...)
		if name == "pre-receive" {
			return 1
		}
		return 0
	}
	body, _ := json.Marshal(req)
	ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(base, "/")+"/internal/hooks/"+name, bytes.NewReader(body))
	if err != nil {
		return fail("anvil: %v", err)
	}
	httpReq.Header.Set("Authorization", "Bearer "+token)
	httpReq.Header.Set("Content-Type", "application/json")
	if id, ok := explorer.TraceIDFromEnv(env); ok {
		httpReq.Header.Set("traceparent", "00-"+id+"-0000000000000001-01")
	}
	res, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		return fail("anvil: cannot reach the server to check this push: %v", err)
	}
	defer res.Body.Close()
	var out response
	if res.StatusCode != http.StatusOK || json.NewDecoder(res.Body).Decode(&out) != nil {
		return fail("anvil: server error checking this push (HTTP %d)", res.StatusCode)
	}
	for _, m := range out.Messages {
		fmt.Fprintln(stderr, m)
	}
	if !out.Allowed && name == "pre-receive" {
		fmt.Fprintln(stderr, out.Message)
		return 1
	}
	return 0
}

// Deps are the internal API's dependencies.
type Deps struct {
	Repos     *repos.Service
	Token     string // the per-process secret hooks must present
	PublicURL string // for links in push messages
}

// InternalAPI returns the http.Handler mounted on the internal listener (/internal/hooks/*).
// Callers must present the per-process token; anything else gets 401.
func InternalAPI(d Deps) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /internal/hooks/{name}", func(w http.ResponseWriter, r *http.Request) {
		got, _ := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
		if subtle.ConstantTimeCompare([]byte(got), []byte(d.Token)) != 1 {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		var req request
		if err := json.NewDecoder(io.LimitReader(r.Body, 8<<20)).Decode(&req); err != nil {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		ctx := r.Context()
		log := logging.FromContext(ctx).With("repo_id", req.RepoID, "pusher_id", req.PusherID, "hook", r.PathValue("name"))
		var out response
		switch r.PathValue("name") {
		case "pre-receive":
			allowed, msg, err := d.Repos.CheckPush(ctx, req.RepoID, req.Updates, req.Env)
			if err != nil {
				log.Error("pre-receive check failed", "err", err)
				http.Error(w, "internal error", http.StatusInternalServerError)
				return
			}
			out.Allowed, out.Message = allowed, msg
			if !allowed {
				log.Info("push rejected", "reason", msg)
			}
		case "post-receive":
			msgs, err := d.Repos.AfterPush(ctx, req.RepoID, req.PusherID, req.Updates, d.PublicURL)
			if err != nil {
				log.Error("post-receive failed", "err", err)
				http.Error(w, "internal error", http.StatusInternalServerError)
				return
			}
			out.Allowed, out.Messages = true, msgs
			log.Info("push received", "refs", len(req.Updates))
		default:
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(out)
	})
	return mux
}
