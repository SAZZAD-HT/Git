package httpserver

import (
	"fmt"
	"log/slog"
	"net/http"
	"regexp"
	"runtime/debug"
	"time"

	"anvil/internal/api"
	"anvil/internal/explorer"
	"anvil/internal/logging"
)

// traceparentRE matches a W3C traceparent header: version-traceid-parentid-flags.
var traceparentRE = regexp.MustCompile(`^[0-9a-f]{2}-([0-9a-f]{32})-[0-9a-f]{16}-[0-9a-f]{2}$`)

// Trace gives every request a request id (returned as X-Request-ID) and a trace id
// (taken from a valid W3C traceparent header, otherwise new), and stores a logger carrying
// both in the request context, so every log line for the request has them (NFR-OPS-01).
// Incoming X-Request-ID headers are ignored: clients must not choose ids that appear in logs.
func Trace(base *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			reqID := api.NewRequestID()
			traceID := ""
			if m := traceparentRE.FindStringSubmatch(r.Header.Get("traceparent")); m != nil {
				traceID = m[1]
			} else {
				traceID = explorer.NewTraceID()
			}
			w.Header().Set("X-Request-ID", reqID)
			ctx := api.WithRequestID(r.Context(), reqID)
			ctx = explorer.WithTraceID(ctx, traceID)
			ctx = logging.WithContext(ctx, base.With("request_id", reqID, "trace_id", traceID))
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// Recover turns a panic in a handler into a 500 response with the standard error body and
// logs the stack, so one bug cannot crash the server. http.ErrAbortHandler is re-raised
// because net/http uses it on purpose to abort a response.
func Recover(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			p := recover()
			if p == nil {
				return
			}
			if p == http.ErrAbortHandler {
				panic(p)
			}
			logging.FromContext(r.Context()).Error("panic in handler", "panic", fmt.Sprint(p), "stack", string(debug.Stack()))
			api.WriteError(w, r, fmt.Errorf("panic: %v", p))
		}()
		next.ServeHTTP(w, r)
	})
}

// statusRecorder captures the status code and body size for the access log.
type statusRecorder struct {
	http.ResponseWriter
	status int
	bytes  int64
}

func (s *statusRecorder) WriteHeader(code int) {
	if s.status == 0 {
		s.status = code
	}
	s.ResponseWriter.WriteHeader(code)
}

func (s *statusRecorder) Write(b []byte) (int, error) {
	if s.status == 0 {
		s.status = http.StatusOK
	}
	n, err := s.ResponseWriter.Write(b)
	s.bytes += int64(n)
	return n, err
}

// Unwrap lets http.ResponseController reach Flush and deadlines on the real writer
// (needed for SSE log streaming and long git transfers).
func (s *statusRecorder) Unwrap() http.ResponseWriter { return s.ResponseWriter }

// AccessLog writes one structured line per request after it completes: method, path,
// status, bytes and duration. Query strings are not logged because they may hold tokens.
func AccessLog(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w}
		next.ServeHTTP(rec, r)
		if rec.status == 0 {
			rec.status = http.StatusOK
		}
		logging.FromContext(r.Context()).Info("http request",
			"method", r.Method, "path", r.URL.Path, "status", rec.status,
			"bytes", rec.bytes, "duration_ms", time.Since(start).Milliseconds())
	})
}

// SecurityHeaders sets the headers from docs/reference/10-security.md on every response:
// a strict Content-Security-Policy, no MIME sniffing, no framing, no referrer leakage
// across origins.
func SecurityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("Content-Security-Policy", "default-src 'self'; script-src 'self'; object-src 'none'; frame-ancestors 'none'; img-src 'self' data:")
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("X-Frame-Options", "DENY")
		h.Set("Referrer-Policy", "strict-origin-when-cross-origin")
		next.ServeHTTP(w, r)
	})
}
