// Package explorer powers the Architecture Explorer: it records spans as requests move
// through Anvil's components (HTTP, auth, git, store, queue, CI) so the web UI can show
// live how a forge works (FR-EXP-01..05, docs/reference/03-architecture-overview.md §7).
//
// P1 ships the API skeleton only: StartSpan and Span.End are cheap no-ops that still
// carry a trace id through context, so every package can be instrumented now and the
// recorder, topology and SSE stream arrive in P7 without touching callers.
package explorer
