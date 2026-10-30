// Package logging builds the process-wide structured logger and carries
// request-scoped loggers through context.Context.
//
// Every log line for a request carries request_id and trace_id (NFR-OPS-01): the
// httpserver middleware attaches them with WithContext, and code logs through
// FromContext(ctx). The package uses only log/slog; it must not import any other
// Anvil package except config.
package logging
