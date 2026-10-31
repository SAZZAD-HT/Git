package errs

import (
	"errors"
	"sort"
	"strings"
)

// Domain error kinds. Each maps to one HTTP status in internal/httpserver.
var (
	// NotFound: the object does not exist, or the caller may not know it exists (HTTP 404).
	NotFound = errors.New("not found")
	// Forbidden: the caller is known but not allowed to do this (HTTP 403).
	Forbidden = errors.New("forbidden")
	// Unauthenticated: no or invalid credentials (HTTP 401).
	Unauthenticated = errors.New("unauthenticated")
	// Conflict: the request clashes with current state, e.g. a name is taken or a ref moved (HTTP 409).
	Conflict = errors.New("conflict")
	// Invalid: the input is well-formed but fails validation (HTTP 422).
	Invalid = errors.New("validation failed")
	// RateLimited: the caller exceeded a rate limit (HTTP 429).
	RateLimited = errors.New("rate limited")
)

// FieldErrors is an errs.Invalid error that lists a message for each bad field.
// The key is the field name as the client sent it (snake_case for API input,
// "section.key" for configuration).
type FieldErrors map[string]string

// Error joins every field message in key order, so the text is stable in tests and logs.
func (f FieldErrors) Error() string {
	keys := make([]string, 0, len(f))
	for k := range f {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, k+": "+f[k])
	}
	return "validation failed: " + strings.Join(parts, "; ")
}

// Is makes errors.Is(fieldErrors, errs.Invalid) true.
func (f FieldErrors) Is(target error) bool { return target == Invalid }

// OrNil returns f as an error, or nil when f has no entries. It avoids the
// typed-nil-in-interface trap when a function returns FieldErrors as error.
func (f FieldErrors) OrNil() error {
	if len(f) == 0 {
		return nil
	}
	return f
}
