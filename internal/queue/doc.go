// Package queue runs durable background work: pipeline evaluation after a push,
// mergeability checks, repository maintenance.
//
// Tasks are rows in the tasks table, so they survive a restart. A worker leases a task
// for a fixed time; if the worker crashes, the lease expires and another worker retries
// it. Failed tasks are retried with exponential backoff up to MaxAttempts
// (docs/reference/05-data-model.md §3.6). Handlers must therefore be idempotent.
package queue
