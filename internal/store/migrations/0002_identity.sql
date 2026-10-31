-- 0002_identity: users, browser sessions and personal access tokens (docs/reference/05-data-model.md §3.1).
-- Secrets are never stored in plain text: session ids and tokens are SHA-256 hashes,
-- passwords are argon2id hashes. Timestamps are unix milliseconds.

CREATE TABLE users (
    id             INTEGER PRIMARY KEY,
    username       TEXT    NOT NULL,           -- display form
    username_lower TEXT    NOT NULL UNIQUE,    -- lookups are case-insensitive
    email          TEXT    NOT NULL UNIQUE,    -- stored lower-cased
    full_name      TEXT    NOT NULL DEFAULT '',
    password_hash  TEXT    NOT NULL,           -- $argon2id$v=19$m=65536,t=3,p=2$salt$hash
    is_admin       INTEGER NOT NULL DEFAULT 0 CHECK (is_admin IN (0, 1)),
    is_active      INTEGER NOT NULL DEFAULT 1 CHECK (is_active IN (0, 1)),
    created_at     INTEGER NOT NULL,
    updated_at     INTEGER NOT NULL
) STRICT;

CREATE TABLE sessions (
    id_hash      TEXT    PRIMARY KEY,          -- SHA-256 of the cookie value, hex
    user_id      INTEGER NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    csrf_token   TEXT    NOT NULL,
    ip           TEXT    NOT NULL DEFAULT '',
    user_agent   TEXT    NOT NULL DEFAULT '',
    created_at   INTEGER NOT NULL,
    last_seen_at INTEGER NOT NULL,
    expires_at   INTEGER NOT NULL              -- sliding; never later than created_at + 30 days
) STRICT;
CREATE INDEX sessions_user ON sessions (user_id);

CREATE TABLE access_tokens (
    id           INTEGER PRIMARY KEY,
    user_id      INTEGER NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    name         TEXT    NOT NULL,
    token_prefix TEXT    NOT NULL,             -- first 12 characters, shown in the UI
    token_hash   TEXT    NOT NULL UNIQUE,      -- SHA-256 of the full token, hex
    scopes       TEXT    NOT NULL,             -- JSON array: repo:read, repo:write, admin, package:write
    expires_at   INTEGER,                      -- NULL = never expires
    last_used_at INTEGER,
    created_at   INTEGER NOT NULL
) STRICT;
CREATE INDEX access_tokens_user ON access_tokens (user_id);
