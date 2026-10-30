-- 0001_init: system tables needed before any feature (docs/reference/05-data-model.md §3.6).
-- settings: instance settings edited in the admin UI.
-- events:   the durable event log behind internal/events (activity feeds, webhooks, explorer replay).
-- tasks:    the durable background work queue behind internal/queue.

CREATE TABLE settings (
    key   TEXT PRIMARY KEY,
    value TEXT NOT NULL            -- JSON
) STRICT;

CREATE TABLE events (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    type       TEXT    NOT NULL,   -- e.g. git.push, ci.job.finished
    repo_id    INTEGER,            -- NULL for instance-wide events
    actor_kind TEXT    NOT NULL CHECK (actor_kind IN ('user', 'runner', 'system')),
    actor_id   INTEGER,
    payload    TEXT    NOT NULL,   -- JSON
    created_at INTEGER NOT NULL    -- unix seconds
) STRICT;
CREATE INDEX events_repo ON events (repo_id, id);
CREATE INDEX events_type ON events (type, id);

CREATE TABLE tasks (
    id           INTEGER PRIMARY KEY AUTOINCREMENT,
    kind         TEXT    NOT NULL,  -- ci.evaluate_push, pull.check_mergeable, repo.maintenance, ...
    payload      TEXT    NOT NULL,  -- JSON
    status       TEXT    NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'running', 'done', 'failed')),
    attempts     INTEGER NOT NULL DEFAULT 0,
    run_after    INTEGER NOT NULL,  -- earliest start, unix seconds
    locked_until INTEGER NOT NULL DEFAULT 0, -- lease end; a crashed worker's task is retried after it
    last_error   TEXT    NOT NULL DEFAULT '',
    created_at   INTEGER NOT NULL
) STRICT;
CREATE INDEX tasks_ready ON tasks (status, run_after);
