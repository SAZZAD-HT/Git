package logging

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"

	"anvil/internal/config"
)

type ctxKey struct{}

// New builds the process-wide slog.Logger from cfg (level, text or json, file or stderr).
// The returned Closer closes the log file; it is a no-op for stderr. New also installs
// the logger as slog's default so that FromContext has a sensible fallback.
// It returns an error if the log file cannot be opened for appending.
func New(cfg config.LogConfig) (*slog.Logger, io.Closer, error) {
	var level slog.Level
	if err := level.UnmarshalText([]byte(strings.ToUpper(cfg.Level))); err != nil {
		return nil, nil, fmt.Errorf("log.level: %w", err)
	}
	var w io.Writer = os.Stderr
	var closer io.Closer = io.NopCloser(nil)
	if cfg.File != "" {
		f, err := os.OpenFile(cfg.File, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o640)
		if err != nil {
			return nil, nil, fmt.Errorf("open log file: %w", err)
		}
		w, closer = f, f
	}
	l := NewWithWriter(w, cfg.Format, level)
	slog.SetDefault(l)
	return l, closer, nil
}

// NewWithWriter builds a logger writing to w in format "text" or "json". Used by New and by tests.
func NewWithWriter(w io.Writer, format string, level slog.Level) *slog.Logger {
	opts := &slog.HandlerOptions{Level: level}
	if format == "json" {
		return slog.New(slog.NewJSONHandler(w, opts))
	}
	return slog.New(slog.NewTextHandler(w, opts))
}

// FromContext returns the request-scoped logger stored by WithContext (which carries
// request_id and trace_id), or slog.Default() when ctx has none.
func FromContext(ctx context.Context) *slog.Logger {
	if l, ok := ctx.Value(ctxKey{}).(*slog.Logger); ok {
		return l
	}
	return slog.Default()
}

// WithContext returns a copy of ctx carrying logger l.
func WithContext(ctx context.Context, l *slog.Logger) context.Context {
	return context.WithValue(ctx, ctxKey{}, l)
}
