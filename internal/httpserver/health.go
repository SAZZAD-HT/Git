package httpserver

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"net/http"
	"path"
	"strings"
	"time"

	"anvil/internal/errs"
	"anvil/internal/logging"
)

var errNotFound = errs.NotFound

// healthz answers liveness probes: the process is up and serving HTTP.
func healthz(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	io.WriteString(w, "ok\n")
}

// readyz answers readiness probes. It runs every checker with a 3-second limit and returns
// 200 with {"status":"ready"} or 503 with the failing checks (TC-SMK-01).
func readyz(checkers []ReadyChecker) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
		defer cancel()
		checks := map[string]string{}
		ready := true
		for _, c := range checkers {
			if err := c.Ready(ctx); err != nil {
				ready = false
				checks[c.Name()] = err.Error()
				logging.FromContext(ctx).Warn("readiness check failed", "check", c.Name(), "err", err)
			} else {
				checks[c.Name()] = "ok"
			}
		}
		status, code := "ready", http.StatusOK
		if !ready {
			status, code = "not_ready", http.StatusServiceUnavailable
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		w.WriteHeader(code)
		json.NewEncoder(w).Encode(map[string]any{"status": status, "checks": checks})
	}
}

// spaHandler serves the built web app. Real files are served as-is (hashed assets get a
// long cache lifetime); every other path returns index.html so the client router can
// handle it. If the app has not been built, placeholder.html is served instead.
func spaHandler(spa fs.FS) http.Handler {
	index := "index.html"
	if _, err := fs.Stat(spa, index); errors.Is(err, fs.ErrNotExist) {
		index = "placeholder.html"
	}
	files := http.FileServerFS(spa)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		name := strings.TrimPrefix(path.Clean(r.URL.Path), "/")
		if name != "" && name != index {
			if fi, err := fs.Stat(spa, name); err == nil && !fi.IsDir() {
				if strings.HasPrefix(name, "assets/") { // Vite puts content-hashed files here
					w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
				}
				files.ServeHTTP(w, r)
				return
			}
		}
		b, err := fs.ReadFile(spa, index)
		if err != nil {
			http.Error(w, "web app missing from this build", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-cache")
		w.Write(b)
	})
}
