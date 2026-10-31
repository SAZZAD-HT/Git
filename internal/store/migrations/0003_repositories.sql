-- 0003_repositories: repositories, collaborators and branch protection
-- (docs/reference/05-data-model.md §3.2). The bare repository on disk lives at
-- <data_dir>/repos/<owner_lower>/<name_lower>.git (docs/reference/04 §1).

CREATE TABLE repositories (
    id             INTEGER PRIMARY KEY,
    owner_id       INTEGER NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    name           TEXT    NOT NULL,
    name_lower     TEXT    NOT NULL,
    description    TEXT    NOT NULL DEFAULT '',
    visibility     TEXT    NOT NULL DEFAULT 'public' CHECK (visibility IN ('public', 'private')),
    default_branch TEXT    NOT NULL DEFAULT 'main',
    is_empty       INTEGER NOT NULL DEFAULT 1 CHECK (is_empty IN (0, 1)),
    fork_of_id     INTEGER REFERENCES repositories (id) ON DELETE SET NULL,
    next_index     INTEGER NOT NULL DEFAULT 1,  -- next issue/PR number
    size_bytes     INTEGER NOT NULL DEFAULT 0,
    created_at     INTEGER NOT NULL,
    updated_at     INTEGER NOT NULL,
    UNIQUE (owner_id, name_lower)
) STRICT;

CREATE TABLE collaborators (
    repo_id    INTEGER NOT NULL REFERENCES repositories (id) ON DELETE CASCADE,
    user_id    INTEGER NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    role       TEXT    NOT NULL CHECK (role IN ('read', 'write', 'admin')),
    created_at INTEGER NOT NULL,
    PRIMARY KEY (repo_id, user_id)
) STRICT;
CREATE INDEX collaborators_user ON collaborators (user_id);

CREATE TABLE protected_branches (
    id                   INTEGER PRIMARY KEY,
    repo_id              INTEGER NOT NULL REFERENCES repositories (id) ON DELETE CASCADE,
    pattern              TEXT    NOT NULL,             -- glob: main, release/*
    allow_force_push     INTEGER NOT NULL DEFAULT 0 CHECK (allow_force_push IN (0, 1)),
    allow_delete         INTEGER NOT NULL DEFAULT 0 CHECK (allow_delete IN (0, 1)),
    require_pull_request INTEGER NOT NULL DEFAULT 0 CHECK (require_pull_request IN (0, 1)),
    required_checks      TEXT    NOT NULL DEFAULT '[]', -- JSON array of job names
    created_at           INTEGER NOT NULL,
    UNIQUE (repo_id, pattern)
) STRICT;
