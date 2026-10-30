package api

import (
	"context"
	"crypto/rand"
	"encoding/binary"
	"errors"
	"net/http"
	"time"

	"anvil/internal/errs"
	"anvil/internal/logging"
)

// errorBody is the error shape every endpoint returns (docs/reference/06-api-spec.md §1).
type errorBody struct {
	Error struct {
		Code    string         `json:"code"`
		Message string         `json:"message"`
		Details map[string]any `json:"details"`
	} `json:"error"`
	RequestID string `json:"request_id"`
}

// mapping is the single table from domain error kinds to HTTP. Order matters only if an
// error wraps several kinds; the first match wins.
var mapping = []struct {
	kind   error
	status int
	code   string
}{
	{errs.NotFound, http.StatusNotFound, "not_found"},
	{errs.Unauthenticated, http.StatusUnauthorized, "unauthenticated"},
	{errs.Forbidden, http.StatusForbidden, "forbidden"},
	{errs.Conflict, http.StatusConflict, "conflict"},
	{errs.Invalid, http.StatusUnprocessableEntity, "validation_failed"},
	{errs.RateLimited, http.StatusTooManyRequests, "rate_limited"},
}

// WriteError writes err as the standard JSON error body. Domain errors keep their message;
// anything else becomes a 500 "internal" whose real message is logged with the request id
// but never sent to the client, so internals (paths, SQL) do not leak.
func WriteError(w http.ResponseWriter, r *http.Request, err error) {
	var body errorBody
	body.RequestID = RequestID(r.Context())
	body.Error.Details = map[string]any{}
	status := http.StatusInternalServerError
	body.Error.Code, body.Error.Message = "internal", "Internal error. Quote request id "+body.RequestID+" when reporting it."
	for _, m := range mapping {
		if errors.Is(err, m.kind) {
			status, body.Error.Code, body.Error.Message = m.status, m.code, err.Error()
			break
		}
	}
	var fe errs.FieldErrors
	if errors.As(err, &fe) {
		body.Error.Details["fields"] = fe
	}
	if status == http.StatusInternalServerError {
		logging.FromContext(r.Context()).Error("internal error", "err", err)
	}
	writeJSON(w, r, status, body)
}

type requestIDKey struct{}

// WithRequestID returns a copy of ctx carrying request id id.
func WithRequestID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, requestIDKey{}, id)
}

// RequestID returns the request id stored in ctx, or "".
func RequestID(ctx context.Context) string {
	id, _ := ctx.Value(requestIDKey{}).(string)
	return id
}

const crockford = "0123456789ABCDEFGHJKMNPQRSTVWXYZ"

// NewRequestID returns a ULID: 26 characters that sort by creation time (48-bit
// millisecond timestamp + 80 random bits, Crockford base32), e.g. "01JBX3K9…".
// Time-sortable ids make it easy to find a request in logs.
func NewRequestID() string {
	var b [16]byte
	binary.BigEndian.PutUint64(b[:8], uint64(time.Now().UnixMilli())<<16)
	_, _ = rand.Read(b[6:])
	// Encode 128 bits as 26 base32 digits, most significant first (the first digit holds 3 bits).
	hi, lo := binary.BigEndian.Uint64(b[:8]), binary.BigEndian.Uint64(b[8:])
	out := make([]byte, 26)
	for i := 25; i >= 0; i-- {
		out[i] = crockford[lo&31]
		lo = lo>>5 | hi<<59
		hi >>= 5
	}
	return string(out)
}
