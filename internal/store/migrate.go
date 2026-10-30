package store

import (
	"context"
	"embed"
	"fmt"
	"io/fs"
	"sort"
	"strconv"
	"strings"
	"time"
)

//go:embed migrations/*.sql
var migrationFiles embed.FS

type migration struct {
	version int
	name    string // file name, e.g. "0001_init.sql"
	sql     string
}

// loadMigrations returns the embedded migrations sorted by version.
// File names must start with a unique number followed by "_".
func loadMigrations() ([]migration, error) {
	entries, err := fs.ReadDir(migrationFiles, "migrations")
	if err != nil {
		return nil, err
	}
	var out []migration
	seen := map[int]string{}
	for _, e := range entries {
		num, _, ok := strings.Cut(e.Name(), "_")
		v, err := strconv.Atoi(num)
		if !ok || err != nil {
			return nil, fmt.Errorf("migration %s: name must start with a number and _", e.Name())
		}
		if prev, dup := seen[v]; dup {
			return nil, fmt.Errorf("migrations %s and %s share version %d", prev, e.Name(), v)
		}
		seen[v] = e.Name()
		b, err := migrationFiles.ReadFile("migrations/" + e.Name())
		if err != nil {
			return nil, err
		}
		out = append(out, migration{version: v, name: e.Name(), sql: string(b)})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].version < out[j].version })
	return out, nil
}

const createVersionTable = `CREATE TABLE IF NOT EXISTS schema_migrations (
	version    INTEGER PRIMARY KEY,
	applied_at INTEGER NOT NULL
)`

// Migrate applies every embedded migration newer than the recorded version, each in its
// own transaction, and returns the file names it applied (or, with dryRun, would apply).
//
// A failed migration is rolled back and stops the run; earlier ones stay applied. It
// returns an error if the database records a version this binary does not know, which
// means the database was migrated by a newer Anvil (downgrades are not supported).
func (db *DB) Migrate(ctx context.Context, dryRun bool) ([]string, error) {
	pending, err := db.pendingMigrations(ctx)
	if err != nil {
		return nil, err
	}
	names := make([]string, 0, len(pending))
	for _, m := range pending {
		names = append(names, m.name)
		if dryRun {
			continue
		}
		err := db.Tx(ctx, func(tx *Tx) error {
			if _, err := tx.tx.ExecContext(ctx, m.sql); err != nil {
				return err
			}
			_, err := tx.tx.ExecContext(ctx, "INSERT INTO schema_migrations (version, applied_at) VALUES (?, ?)", m.version, time.Now().Unix())
			return err
		})
		if err != nil {
			return names[:len(names)-1], fmt.Errorf("migration %s: %w", m.name, err)
		}
	}
	return names, nil
}

// PendingMigrations returns the file names of migrations not yet applied.
// /readyz reports not-ready while this is non-empty.
func (db *DB) PendingMigrations(ctx context.Context) ([]string, error) {
	pending, err := db.pendingMigrations(ctx)
	if err != nil {
		return nil, err
	}
	names := make([]string, len(pending))
	for i, m := range pending {
		names[i] = m.name
	}
	return names, nil
}

func (db *DB) pendingMigrations(ctx context.Context) ([]migration, error) {
	all, err := loadMigrations()
	if err != nil {
		return nil, err
	}
	if _, err := db.write.ExecContext(ctx, createVersionTable); err != nil {
		return nil, fmt.Errorf("create schema_migrations: %w", err)
	}
	rows, err := db.read.QueryContext(ctx, "SELECT version FROM schema_migrations")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	applied := map[int]bool{}
	for rows.Next() {
		var v int
		if err := rows.Scan(&v); err != nil {
			return nil, err
		}
		applied[v] = true
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	known := map[int]bool{}
	var pending []migration
	for _, m := range all {
		known[m.version] = true
		if !applied[m.version] {
			pending = append(pending, m)
		}
	}
	for v := range applied {
		if !known[v] {
			return nil, fmt.Errorf("database has migration %d, which this version of Anvil does not know: it was upgraded by a newer release; restore a backup instead of downgrading", v)
		}
	}
	return pending, nil
}
