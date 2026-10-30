package explorer

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"log/slog"
	"strings"
)

// EnvTraceID passes a trace from the server to `anvil hook` through git's environment.
const EnvTraceID = "ANVIL_TRACE_ID"

type traceKey struct{}

// Span is one timed operation inside a trace. A nil *Span is valid and does nothing.
type Span struct {
	TraceID   string
	Component string
	Operation string
}

// StartSpan begins a trace span for component/operation, returning a ctx that carries it.
// Cheap no-op when the Explorer is disabled. A new trace id is created if ctx has none.
func StartSpan(ctx context.Context, component, operation string, attrs ...slog.Attr) (context.Context, *Span) {
	id, ok := TraceID(ctx)
	if !ok {
		id = NewTraceID()
		ctx = WithTraceID(ctx, id)
	}
	return ctx, &Span{TraceID: id, Component: component, Operation: operation}
}

// End finishes the span, recording err if the operation failed. Safe on a nil Span.
func (s *Span) End(err error) {}

// NewTraceID returns a random 16-byte trace id in lower-case hex (W3C trace-context format).
func NewTraceID() string {
	var b [16]byte
	_, _ = rand.Read(b[:]) // crypto/rand.Read never fails on supported platforms
	return hex.EncodeToString(b[:])
}

// WithTraceID returns a copy of ctx carrying trace id id.
func WithTraceID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, traceKey{}, id)
}

// TraceID returns the trace id carried by ctx.
func TraceID(ctx context.Context) (string, bool) {
	id, ok := ctx.Value(traceKey{}).(string)
	return id, ok && id != ""
}

// TraceIDFromEnv restores a trace from ANVIL_TRACE_ID (used by `anvil hook`).
// env is a KEY=VALUE list as returned by os.Environ. It accepts only 32 hex characters.
func TraceIDFromEnv(env []string) (string, bool) {
	for _, kv := range env {
		if v, ok := strings.CutPrefix(kv, EnvTraceID+"="); ok && validTraceID(v) {
			return v, true
		}
	}
	return "", false
}

func validTraceID(s string) bool {
	if len(s) != 32 {
		return false
	}
	_, err := hex.DecodeString(s)
	return err == nil
}
