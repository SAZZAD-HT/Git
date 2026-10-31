// Package events is the in-process event bus. Services publish what happened
// (git.push, pull.opened, ci.job.finished, …) and other services react, so modules
// never call each other's internals (ADR-008, docs/reference/03-architecture-overview.md §6).
//
// Publish first stores the event in the events table, then delivers it to in-process
// subscribers. Delivery never blocks the publisher: a subscriber that falls behind
// receives a gap marker and can replay from the table with store.EventsAfter.
package events
