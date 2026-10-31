// Package api implements the REST API under /api/v1 (contract: api/openapi.yaml,
// docs/reference/06-api-spec.md).
//
// There is one file per resource, each with a mount function registered in Mount.
// Handlers decode input, call a service with the acting user, and encode the result;
// they never decide permissions and never touch the database directly. errors.go is the
// single place where domain errors (internal/errs) become HTTP status codes.
package api
