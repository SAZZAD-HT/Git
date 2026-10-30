// Package store owns the SQL database: opening it, migrating its schema, and every query.
//
// It is the only package that imports database/sql (enforced by internal/archtest).
// SQLite runs through the pure-Go modernc.org/sqlite driver (ADR-005, no cgo) in WAL
// mode with one writer connection and a pool of readers, so writes are serialised in
// the process instead of failing with SQLITE_BUSY.
//
// Reads go through methods on *DB; writes go through methods on *Tx inside DB.Tx.
// Transactions must be short: never call git or do network I/O inside one
// (docs/reference/05-data-model.md §5). Migrations are numbered SQL files embedded from
// migrations/, applied in order at startup and recorded in schema_migrations.
//
// Queries are grouped in one file per table (settings.go, events.go, tasks.go, …).
package store
