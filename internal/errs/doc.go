// Package errs defines the domain error kinds shared by every service.
//
// Services return these sentinels, wrapped with context:
//
//	return fmt.Errorf("repository %s/%s: %w", owner, name, errs.NotFound)
//
// Callers test with errors.Is. The HTTP layer maps each kind to a status code and an
// error code in exactly one place (docs/reference/06-api-spec.md §1); nothing else
// translates errors. Fields lists per-field validation messages for errs.Invalid.
//
// This package must not import anything from Anvil; every layer depends on it.
package errs
