package httpserver

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"net"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"anvil/internal/api"
	"anvil/internal/auth"
	"anvil/internal/config"
	"anvil/internal/git/smarthttp"
)

// ReadyChecker reports whether a dependency is ready to serve traffic. /readyz calls
// every checker; one failure makes the server not-ready.
type ReadyChecker interface {
	// Name identifies the dependency in /readyz output, e.g. "database".
	Name() string
	// Ready returns nil when the dependency is usable.
	Ready(ctx context.Context) error
}

// Deps are the server's dependencies, built by `anvil serve` (the composition root).
type Deps struct {
	Config   *config.Config
	Logger   *slog.Logger
	Checkers []ReadyChecker
	API      api.Deps
	// Auth authenticates API requests. Nil serves the API without authentication (tests of health endpoints).
	Auth *auth.Service
	// Git serves clone/fetch/push; nil disables the git routes.
	Git *smarthttp.Handler
	// Hooks is the internal hook API, mounted on the loopback listener; nil disables it.
	Hooks http.Handler
	// SPA is the built web app (index.html at its root). Paths that are not files fall
	// back to index.html so client-side routes like /alice/demo/pulls work on reload.
	SPA fs.FS
}

// Server is the assembled HTTP server.
type Server struct {
	deps     Deps
	main     *http.Server
	internal *http.Server
}

// New assembles the router: middleware chain, /api/v1, health endpoints and the embedded
// SPA fallback. Git smart HTTP, the runner API and the explorer are mounted here in later phases.
func New(deps Deps) *Server {
	r := chi.NewRouter()
	r.Use(Trace(deps.Logger), Recover, AccessLog, SecurityHeaders)
	r.Get("/healthz", healthz)
	r.Get("/readyz", readyz(deps.Checkers))
	apiNotFound := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		api.WriteError(w, r, fmt.Errorf("no API route %s %s: %w", r.Method, r.URL.Path, errNotFound))
	})
	r.Route("/api/v1", func(r chi.Router) {
		if deps.Auth != nil {
			r.Use(Authenticate(deps.Auth, deps.API.SecureCookies), RateLimit(DefaultRateConfig, deps.API.ClientIP), CSRF)
		}
		r.NotFound(apiNotFound) // otherwise chi answers with plain text inside the sub-router
		r.MethodNotAllowed(apiNotFound)
		api.Mount(r, deps.API)
	})
	r.Handle("/api/*", apiNotFound)
	if deps.Git != nil && deps.Auth != nil {
		mountGit(r, deps)
	}
	r.Handle("/*", spaHandler(deps.SPA))

	// The internal listener (loopback only) serves the hook API and a health check.
	ir := chi.NewRouter()
	ir.Use(Trace(deps.Logger), Recover)
	ir.Get("/healthz", healthz)
	if deps.Hooks != nil {
		ir.Handle("/internal/*", deps.Hooks)
	}

	return &Server{
		deps:     deps,
		main:     newHTTPServer(deps.Config.Server.HTTPAddr, r),
		internal: newHTTPServer(deps.Config.Server.InternalAddr, ir),
	}
}

func newHTTPServer(addr string, h http.Handler) *http.Server {
	return &http.Server{
		Addr:              addr,
		Handler:           h,
		ReadHeaderTimeout: 10 * time.Second, // slowloris protection; bodies (pushes) may be slow, so no ReadTimeout
		IdleTimeout:       2 * time.Minute,
	}
}

// Handler returns the main router, for tests that use httptest.
func (s *Server) Handler() http.Handler { return s.main.Handler }

// Listen opens both listeners without serving yet, so callers learn about a port that is
// already in use before reporting the server as started. It returns the bound addresses.
func (s *Server) Listen() (mainLn, internalLn net.Listener, err error) {
	mainLn, err = net.Listen("tcp", s.main.Addr)
	if err != nil {
		return nil, nil, fmt.Errorf("listen on server.http_addr %s: %w", s.main.Addr, err)
	}
	internalLn, err = net.Listen("tcp", s.internal.Addr)
	if err != nil {
		mainLn.Close()
		return nil, nil, fmt.Errorf("listen on server.internal_addr %s: %w", s.internal.Addr, err)
	}
	return mainLn, internalLn, nil
}

// Serve serves on listeners from Listen and blocks until ctx is done, then shuts down
// gracefully, waiting up to server.shutdown_timeout for in-flight requests. It returns
// the first listener error, or nil after a clean shutdown.
func (s *Server) Serve(ctx context.Context, mainLn, internalLn net.Listener) error {
	cfg := s.deps.Config.Server
	errc := make(chan error, 2)
	go func() {
		if cfg.TLSCert != "" {
			errc <- s.main.ServeTLS(mainLn, cfg.TLSCert, cfg.TLSKey)
		} else {
			errc <- s.main.Serve(mainLn)
		}
	}()
	go func() { errc <- s.internal.Serve(internalLn) }()

	var serveErr error
	select {
	case <-ctx.Done():
	case serveErr = <-errc:
	}
	shutdownCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), cfg.ShutdownTimeout.Duration)
	defer cancel()
	err := errors.Join(s.main.Shutdown(shutdownCtx), s.internal.Shutdown(shutdownCtx))
	if serveErr != nil && !errors.Is(serveErr, http.ErrServerClosed) {
		return serveErr
	}
	return err
}

// Run is Listen followed by Serve.
func (s *Server) Run(ctx context.Context) error {
	m, i, err := s.Listen()
	if err != nil {
		return err
	}
	return s.Serve(ctx, m, i)
}

// mountGit registers the three smart HTTP endpoints for /{owner}/{repo}[.git]/…
// (docs/reference/04 §2). They take precedence over the SPA fallback because chi prefers
// the more specific pattern.
func mountGit(r chi.Router, deps Deps) {
	r.Group(func(r chi.Router) {
		r.Use(AuthenticateGit(deps.Auth), RateLimit(DefaultRateConfig, deps.API.ClientIP))
		r.Get("/{owner}/{repo}/info/refs", func(w http.ResponseWriter, r *http.Request) {
			deps.Git.InfoRefs(w, r, chi.URLParam(r, "owner"), chi.URLParam(r, "repo"))
		})
		for _, svc := range []string{"git-upload-pack", "git-receive-pack"} {
			r.Post("/{owner}/{repo}/"+svc, func(w http.ResponseWriter, r *http.Request) {
				deps.Git.RPC(w, r, chi.URLParam(r, "owner"), chi.URLParam(r, "repo"), svc)
			})
		}
	})
}
