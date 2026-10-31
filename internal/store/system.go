package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"anvil/internal/errs"
)

// ---- settings ----

// Setting returns the JSON value stored for key, or errs.NotFound.
func (q *Queries) Setting(ctx context.Context, key string) (string, error) {
	var v string
	err := q.q.QueryRowContext(ctx, "SELECT value FROM settings WHERE key = ?", key).Scan(&v)
	if errors.Is(err, sql.ErrNoRows) {
		return "", fmt.Errorf("setting %s: %w", key, errs.NotFound)
	}
	return v, err
}

// SetSetting inserts or replaces the JSON value for key.
func (q *Queries) SetSetting(ctx context.Context, key, jsonValue string) error {
	_, err := q.q.ExecContext(ctx, "INSERT INTO settings (key, value) VALUES (?, ?) ON CONFLICT (key) DO UPDATE SET value = excluded.value", key, jsonValue)
	return err
}

// ---- events ----

// EventRow is one row of the events table.
type EventRow struct {
	ID        int64
	Type      string
	RepoID    int64 // 0 for instance-wide events
	ActorKind string
	ActorID   int64
	Payload   string // JSON
	CreatedAt int64
}

// InsertEvent stores ev and returns its new id. ev.ID is ignored.
func (q *Queries) InsertEvent(ctx context.Context, ev EventRow) (int64, error) {
	res, err := q.q.ExecContext(ctx,
		"INSERT INTO events (type, repo_id, actor_kind, actor_id, payload, created_at) VALUES (?, NULLIF(?, 0), ?, NULLIF(?, 0), ?, ?)",
		ev.Type, ev.RepoID, ev.ActorKind, ev.ActorID, ev.Payload, ev.CreatedAt)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// EventsAfter returns up to limit events with id > afterID, oldest first.
// Used to replay events a subscriber missed.
func (q *Queries) EventsAfter(ctx context.Context, afterID int64, limit int) ([]EventRow, error) {
	rows, err := q.q.QueryContext(ctx,
		"SELECT id, type, COALESCE(repo_id, 0), actor_kind, COALESCE(actor_id, 0), payload, created_at FROM events WHERE id > ? ORDER BY id LIMIT ?",
		afterID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []EventRow
	for rows.Next() {
		var e EventRow
		if err := rows.Scan(&e.ID, &e.Type, &e.RepoID, &e.ActorKind, &e.ActorID, &e.Payload, &e.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// ---- tasks ----

// TaskRow is one row of the tasks table.
type TaskRow struct {
	ID        int64
	Kind      string
	Payload   string // JSON
	Attempts  int
	LastError string
}

// InsertTask stores a pending task that may start at runAfter (unix milliseconds) and returns its id.
func (q *Queries) InsertTask(ctx context.Context, kind, payload string, runAfter, now int64) (int64, error) {
	res, err := q.q.ExecContext(ctx,
		"INSERT INTO tasks (kind, payload, run_after, created_at) VALUES (?, ?, ?, ?)", kind, payload, runAfter, now)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// ClaimTask leases the oldest ready task of one of kinds until leaseUntil and increments its
// attempt count. A task is ready when it is pending and run_after has passed, or when it is
// running but its lease expired (its worker crashed). It returns errs.NotFound when no task
// is ready. Must run inside Tx so that two workers cannot claim the same task.
func (q *Queries) ClaimTask(ctx context.Context, kinds []string, now, leaseUntil int64) (*TaskRow, error) {
	if len(kinds) == 0 {
		return nil, fmt.Errorf("claim task: %w", errs.NotFound)
	}
	args := []any{now, now}
	placeholders := ""
	for i, k := range kinds {
		if i > 0 {
			placeholders += ", "
		}
		placeholders += "?"
		args = append(args, k)
	}
	var t TaskRow
	err := q.q.QueryRowContext(ctx, `
		SELECT id, kind, payload, attempts, last_error FROM tasks
		WHERE ((status = 'pending' AND run_after <= ?) OR (status = 'running' AND locked_until <= ?))
		  AND kind IN (`+placeholders+`)
		ORDER BY run_after, id LIMIT 1`, args...).Scan(&t.ID, &t.Kind, &t.Payload, &t.Attempts, &t.LastError)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("claim task: %w", errs.NotFound)
	}
	if err != nil {
		return nil, err
	}
	t.Attempts++
	_, err = q.q.ExecContext(ctx, "UPDATE tasks SET status = 'running', attempts = ?, locked_until = ? WHERE id = ?", t.Attempts, leaseUntil, t.ID)
	return &t, err
}

// CompleteTask marks task id done.
func (q *Queries) CompleteTask(ctx context.Context, id int64) error {
	_, err := q.q.ExecContext(ctx, "UPDATE tasks SET status = 'done', locked_until = 0, last_error = '' WHERE id = ?", id)
	return err
}

// RetryTask records a failure and makes task id pending again from runAfter (unix milliseconds),
// or marks it failed for good when final is true.
func (q *Queries) RetryTask(ctx context.Context, id int64, lastError string, runAfter int64, final bool) error {
	status := "pending"
	if final {
		status = "failed"
	}
	_, err := q.q.ExecContext(ctx, "UPDATE tasks SET status = ?, run_after = ?, locked_until = 0, last_error = ? WHERE id = ?", status, runAfter, lastError, id)
	return err
}

// CountTasks returns how many tasks have the given status. Used for queue-depth stats.
func (q *Queries) CountTasks(ctx context.Context, status string) (int64, error) {
	var n int64
	err := q.q.QueryRowContext(ctx, "SELECT COUNT(*) FROM tasks WHERE status = ?", status).Scan(&n)
	return n, err
}
