-- 0004_issues_pulls: issues, pull requests, comments, reviews and labels
-- (docs/reference/05-data-model.md §3.3). Issues and pull requests share one number
-- sequence per repository (repositories.next_index); a pull request is an issues row with
-- is_pull = 1 plus a pull_requests row.

CREATE TABLE issues (
    id         INTEGER PRIMARY KEY,
    repo_id    INTEGER NOT NULL REFERENCES repositories (id) ON DELETE CASCADE,
    number     INTEGER NOT NULL,
    author_id  INTEGER NOT NULL REFERENCES users (id),
    title      TEXT    NOT NULL,
    body       TEXT    NOT NULL DEFAULT '',   -- Markdown, sanitised when rendered
    state      TEXT    NOT NULL DEFAULT 'open' CHECK (state IN ('open', 'closed')),
    is_pull    INTEGER NOT NULL DEFAULT 0 CHECK (is_pull IN (0, 1)),
    comments   INTEGER NOT NULL DEFAULT 0,    -- comment count, kept for list pages
    closed_at  INTEGER,
    created_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL,
    UNIQUE (repo_id, number)
) STRICT;
CREATE INDEX issues_list ON issues (repo_id, is_pull, state, number);

CREATE TABLE comments (
    id           INTEGER PRIMARY KEY,
    issue_id     INTEGER NOT NULL REFERENCES issues (id) ON DELETE CASCADE,
    author_id    INTEGER REFERENCES users (id),  -- NULL for system comments
    body         TEXT    NOT NULL DEFAULT '',
    kind         TEXT    NOT NULL DEFAULT 'comment' CHECK (kind IN ('comment', 'review', 'system')),
    review_state TEXT    CHECK (review_state IN ('comment', 'approve', 'request_changes')),
    review_id    INTEGER REFERENCES comments (id) ON DELETE CASCADE, -- inline comment's review
    commit_sha   TEXT,                            -- inline review comments only
    path         TEXT,
    line         INTEGER,
    side         TEXT    CHECK (side IN ('old', 'new')),
    created_at   INTEGER NOT NULL,
    updated_at   INTEGER NOT NULL
) STRICT;
CREATE INDEX comments_issue ON comments (issue_id, id);

CREATE TABLE labels (
    id      INTEGER PRIMARY KEY,
    repo_id INTEGER NOT NULL REFERENCES repositories (id) ON DELETE CASCADE,
    name    TEXT    NOT NULL,
    color   TEXT    NOT NULL DEFAULT '#6b7280',  -- #rrggbb
    UNIQUE (repo_id, name)
) STRICT;

CREATE TABLE issue_labels (
    issue_id INTEGER NOT NULL REFERENCES issues (id) ON DELETE CASCADE,
    label_id INTEGER NOT NULL REFERENCES labels (id) ON DELETE CASCADE,
    PRIMARY KEY (issue_id, label_id)
) STRICT;

CREATE TABLE pull_requests (
    id               INTEGER PRIMARY KEY,
    issue_id         INTEGER NOT NULL UNIQUE REFERENCES issues (id) ON DELETE CASCADE,
    base_repo_id     INTEGER NOT NULL REFERENCES repositories (id) ON DELETE CASCADE,
    base_branch      TEXT    NOT NULL,
    head_repo_id     INTEGER NOT NULL REFERENCES repositories (id) ON DELETE CASCADE,
    head_branch      TEXT    NOT NULL,
    head_sha         TEXT    NOT NULL,          -- current head commit
    base_sha         TEXT    NOT NULL DEFAULT '', -- base tip at the last mergeability check
    merge_status     TEXT    NOT NULL DEFAULT 'unknown' CHECK (merge_status IN ('unknown', 'checking', 'clean', 'conflict')),
    conflict_files   TEXT    NOT NULL DEFAULT '[]', -- JSON array
    merged_at        INTEGER,
    merged_by        INTEGER REFERENCES users (id),
    merge_commit_sha TEXT
) STRICT;
CREATE INDEX pull_requests_head ON pull_requests (head_repo_id, head_branch);
CREATE INDEX pull_requests_base ON pull_requests (base_repo_id, base_branch);
