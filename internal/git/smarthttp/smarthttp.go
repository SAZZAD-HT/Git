package smarthttp

import (
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	"anvil/internal/auth"
	"anvil/internal/errs"
	"anvil/internal/explorer"
	"anvil/internal/git/gitcmd"
	"anvil/internal/git/hooks"
	"anvil/internal/logging"
	"anvil/internal/repos"
)

// Deps are the handler's dependencies.
type Deps struct {
	Repos         *repos.Service
	InternalURL   string // e.g. http://127.0.0.1:3001, passed to hooks
	InternalToken string // per-process secret for the hook API
	MaxPushSize   int64  // largest receive-pack body accepted (git.max_push_size)
	// WriteError writes an API-style error for failures that are not git protocol errors.
	WriteError func(w http.ResponseWriter, r *http.Request, err error)
}

// Handler serves the Git smart HTTP protocol for /{owner}/{repo}[.git]/*.
type Handler struct{ d Deps }

// New returns a Handler.
func New(d Deps) *Handler { return &Handler{d: d} }

// service maps the protocol's service names to git subcommands and the access they need.
var services = map[string]struct {
	sub   string
	level auth.Level
}{
	"git-upload-pack":  {"upload-pack", auth.LevelRead},
	"git-receive-pack": {"receive-pack", auth.LevelWrite},
}

// InfoRefs serves GET /{owner}/{repo}/info/refs?service=git-upload-pack|git-receive-pack:
// the ref advertisement that starts every clone, fetch and push.
func (h *Handler) InfoRefs(w http.ResponseWriter, r *http.Request, owner, name string) {
	svc := r.URL.Query().Get("service")
	spec, ok := services[svc]
	if !ok {
		// Dumb-HTTP clients are not supported; tell them plainly.
		http.Error(w, "only the smart HTTP protocol is supported (git 1.6.6 or newer)", http.StatusForbidden)
		return
	}
	path, repoID, ok := h.authorize(w, r, owner, name, spec.level)
	if !ok {
		return
	}
	ctx, span := explorer.StartSpan(r.Context(), "smarthttp", "info/refs "+svc)
	proto := r.Header.Get("Git-Protocol")
	cmd := gitcmd.New(spec.sub, "--stateless-rpc", "--advertise-refs", path).WithEnv(protocolEnv(proto)...)
	_ = repoID
	w.Header().Set("Content-Type", "application/x-"+svc+"-advertisement")
	noCache(w)
	// Protocol v0/v1 responses start with a "# service=…" pkt-line and a flush; v2 does not
	// (same rule as git-http-backend).
	if !strings.Contains(proto, "version=2") {
		writePktLine(w, "# service="+svc+"\n")
		io.WriteString(w, "0000")
	}
	err := cmd.Stream(ctx, w)
	span.End(err)
	if err != nil {
		logging.FromContext(ctx).Error("git advertise-refs failed", "err", err)
	}
}

// RPC serves POST /{owner}/{repo}/git-upload-pack and /git-receive-pack, streaming the
// request body into git and git's output back to the client.
func (h *Handler) RPC(w http.ResponseWriter, r *http.Request, owner, name, svc string) {
	spec, ok := services[svc]
	if !ok {
		http.NotFound(w, r)
		return
	}
	if r.Header.Get("Content-Type") != "application/x-"+svc+"-request" {
		http.Error(w, "unexpected content type", http.StatusUnsupportedMediaType)
		return
	}
	path, repoID, ok := h.authorize(w, r, owner, name, spec.level)
	if !ok {
		return
	}
	ctx, span := explorer.StartSpan(r.Context(), "smarthttp", svc)
	body := io.Reader(r.Body)
	if svc == "git-receive-pack" && h.d.MaxPushSize > 0 {
		body = http.MaxBytesReader(w, r.Body, h.d.MaxPushSize)
	}
	body, err := decodeBody(r.Header.Get("Content-Encoding"), body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	env := protocolEnv(r.Header.Get("Git-Protocol"))
	if svc == "git-receive-pack" {
		env = append(env, hookEnv(r, repoID, h.d)...)
	}
	w.Header().Set("Content-Type", "application/x-"+svc+"-result")
	noCache(w)
	err = gitcmd.New(spec.sub, "--stateless-rpc", path).WithEnv(env...).WithStdin(body).Stream(ctx, w)
	span.End(err)
	if err != nil {
		// Headers are already sent; git's own error is in the stream. Log for the operator.
		logging.FromContext(ctx).Warn("git "+spec.sub+" failed", "err", err)
	}
}

// authorize resolves the repository and checks the caller's access. On failure it writes
// the response (401 challenge for anonymous callers, otherwise the API error) and returns false.
func (h *Handler) authorize(w http.ResponseWriter, r *http.Request, owner, name string, level auth.Level) (path string, repoID int64, ok bool) {
	actor := auth.ActorFrom(r.Context())
	repo, err := h.d.Repos.GetForAccess(r.Context(), actor, owner, strings.TrimSuffix(name, ".git"), level)
	if err != nil {
		if actor.IsAnonymous() && (errors.Is(err, errs.NotFound) || errors.Is(err, errs.Forbidden)) {
			w.Header().Set("WWW-Authenticate", `Basic realm="Anvil"`)
			http.Error(w, "authentication required: use your username and a personal access token", http.StatusUnauthorized)
			return "", 0, false
		}
		h.d.WriteError(w, r, err)
		return "", 0, false
	}
	path, err = h.d.Repos.PathFor(repo.OwnerName, repo.Name)
	if err != nil {
		h.d.WriteError(w, r, err)
		return "", 0, false
	}
	return path, repo.ID, true
}

// hookEnv builds the ANVIL_* variables receive-pack passes to the hooks, so they know the
// repository and pusher without re-authenticating (docs/reference/04 §3.1).
func hookEnv(r *http.Request, repoID int64, d Deps) []string {
	actor := auth.ActorFrom(r.Context())
	env := []string{
		hooks.EnvRepoID + "=" + strconv.FormatInt(repoID, 10),
		hooks.EnvPusherID + "=" + strconv.FormatInt(actor.UserID, 10),
		hooks.EnvInternalURL + "=" + d.InternalURL,
		hooks.EnvInternalToken + "=" + d.InternalToken,
	}
	if id, ok := explorer.TraceID(r.Context()); ok {
		env = append(env, explorer.EnvTraceID+"="+id)
	}
	return env
}

// protocolEnv passes the Git-Protocol header to git as GIT_PROTOCOL, enabling protocol v2.
// Only the documented key=value form is forwarded.
func protocolEnv(header string) []string {
	if header == "" || strings.ContainsAny(header, "\n\r\x00") || len(header) > 256 {
		return nil
	}
	return []string{"GIT_PROTOCOL=" + header}
}

// decodeBody undoes Content-Encoding: gzip, which git clients use for large requests.
func decodeBody(encoding string, body io.Reader) (io.Reader, error) {
	switch encoding {
	case "", "identity":
		return body, nil
	case "gzip", "x-gzip":
		zr, err := gzip.NewReader(body)
		if err != nil {
			return nil, fmt.Errorf("invalid gzip body: %w", err)
		}
		return zr, nil
	}
	return nil, fmt.Errorf("unsupported Content-Encoding %q", encoding)
}

// writePktLine writes s as one git pkt-line: four hex digits giving the length including
// themselves, then the data (docs/reference/04 §2.2).
func writePktLine(w io.Writer, s string) {
	fmt.Fprintf(w, "%04x%s", len(s)+4, s)
}

func noCache(w http.ResponseWriter) {
	w.Header().Set("Cache-Control", "no-cache, max-age=0, must-revalidate")
	w.Header().Set("Expires", "Fri, 01 Jan 1980 00:00:00 GMT")
	w.Header().Set("Pragma", "no-cache")
}
