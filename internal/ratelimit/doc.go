// Package ratelimit counts events per key in fixed time windows, in memory.
//
// It backs the API rate limits (per IP for anonymous requests, per user for signed-in
// ones) and the stricter sign-in failure limit (docs/reference/06 §1, 10 §3). Counts are
// per process and reset on restart, which is acceptable for a single-server forge.
package ratelimit
