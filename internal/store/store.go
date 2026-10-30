package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	_ "modernc.org/sqlite" // registers the "sqlite" driver

	"anvil/internal/config"
)

// querier is the part of *sql.DB and *sql.Tx that query methods need.
type querier interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

// Queries holds the query methods shared by DB (reads) and Tx (writes).
type Queries struct{ q querier }

// DB wraps the SQL connection pools. SQLite uses one writer connection and a read pool.
// It is safe for concurrent use.
type DB struct {
	*Queries // reads, through the reader pool
	write    *sql.DB
	read     *sql.DB
	path     string
}

// Tx is a write transaction. Its methods are only valid inside the function passed to DB.Tx.
type Tx struct {
	*Queries
	tx *sql.Tx
}

// Open opens the database described by cfg and enables foreign keys, WAL mode and a
// busy timeout on every connection. dataDir is used when cfg.DSN is empty; the parent
// directory is created if missing. It does not run migrations; call Migrate.
// It returns an error if the file cannot be opened or is not a SQLite database.
func Open(ctx context.Context, cfg config.DatabaseConfig, dataDir string) (*DB, error) {
	if cfg.Driver != "sqlite" {
		return nil, fmt.Errorf("database driver %q is not supported", cfg.Driver)
	}
	path := cfg.DSN
	if path == "" {
		path = filepath.Join(dataDir, "anvil.db")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return nil, fmt.Errorf("create database directory: %w", err)
	}
	// Pragmas are applied by the driver to every new connection in the pool.
	dsn := path + "?_pragma=foreign_keys(1)&_pragma=journal_mode(WAL)&_pragma=busy_timeout(10000)&_pragma=synchronous(NORMAL)&_txlock=immediate"
	write, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	write.SetMaxOpenConns(1) // SQLite allows one writer; queue writers here instead of in SQLITE_BUSY retries
	read, err := sql.Open("sqlite", dsn)
	if err != nil {
		write.Close()
		return nil, err
	}
	read.SetMaxOpenConns(max(4, runtime.NumCPU()))
	db := &DB{Queries: &Queries{q: read}, write: write, read: read, path: path}
	if err := db.Ping(ctx); err != nil {
		db.Close()
		return nil, fmt.Errorf("open database %s: %w", path, err)
	}
	return db, nil
}

// Path returns the database file path.
func (db *DB) Path() string { return db.path }

// Ping checks that both pools can reach the database. Used by /readyz.
func (db *DB) Ping(ctx context.Context) error {
	if err := db.write.PingContext(ctx); err != nil {
		return err
	}
	var one int
	return db.read.QueryRowContext(ctx, "SELECT 1").Scan(&one)
}

// Tx runs fn in a write transaction, committing if fn returns nil and rolling back
// otherwise (including when fn panics, after which the panic continues).
// fn must not call git or do network I/O (docs/reference/05 §5); it blocks other writers.
// The error from fn is returned unchanged so callers can use errors.Is on it.
func (db *DB) Tx(ctx context.Context, fn func(tx *Tx) error) (err error) {
	sqlTx, err := db.write.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer func() {
		if p := recover(); p != nil {
			_ = sqlTx.Rollback()
			panic(p)
		}
		if err != nil {
			_ = sqlTx.Rollback()
		}
	}()
	if err = fn(&Tx{Queries: &Queries{q: sqlTx}, tx: sqlTx}); err != nil {
		return err
	}
	if err = sqlTx.Commit(); err != nil {
		return fmt.Errorf("commit: %w", err)
	}
	return nil
}

// IntegrityCheck runs SQLite's quick_check and returns an error listing the problems it finds.
// It reads the whole database, so it is slow on large instances; `anvil doctor` uses it.
func (db *DB) IntegrityCheck(ctx context.Context) error {
	rows, err := db.read.QueryContext(ctx, "PRAGMA quick_check")
	if err != nil {
		return err
	}
	defer rows.Close()
	var problems []string
	for rows.Next() {
		var s string
		if err := rows.Scan(&s); err != nil {
			return err
		}
		if s != "ok" {
			problems = append(problems, s)
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}
	if len(problems) > 0 {
		return fmt.Errorf("integrity check failed: %v", problems)
	}
	return nil
}

// Close closes all connections. The DB must not be used afterwards.
func (db *DB) Close() error {
	return errors.Join(db.read.Close(), db.write.Close())
}
