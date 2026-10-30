// Package httpserver owns Anvil's HTTP listeners: the middleware chain, health
// endpoints, mounting of /api/v1 and the embedded single-page app.
//
// Two listeners run: the main one (server.http_addr) for browsers, git clients and
// runners, and the internal one (server.internal_addr, loopback only) for git hooks.
// Run shuts both down gracefully when its context ends, waiting up to
// server.shutdown_timeout for in-flight requests (docs/reference/03 §4.3 step 7).
//
// The package decides nothing about permissions; services do (docs/reference/14 §3).
// Middleware order, outermost first: Trace (request_id, trace_id, request logger),
// Recover, AccessLog, SecurityHeaders.
package httpserver
