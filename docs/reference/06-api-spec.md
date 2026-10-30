# 06 — HTTP API Specification

The React frontend uses **only** this public API. There is no hidden "web-only" API, so anything the UI can do, a script can do too.

There are four API surfaces:

| Surface | Prefix | Used by | Auth |
|---|---|---|---|
| Public REST API | `/api/v1` | SPA, scripts, CLI | Session cookie + CSRF, or `Authorization: Bearer <PAT>` |
| Git smart HTTP | `/{owner}/{repo}.git/…` | `git` | HTTP Basic (user + PAT, or job token) — see [04](04-git-hosting.md) |
| Runner API | `/api/runner/v1` | `anvil-runner` | Runner token or job token — see [07](07-ci-cd-design.md) |
| Internal API | `/internal` | `anvil hook` | `ANVIL_INTERNAL_TOKEN`; bound to `127.0.0.1` only |

An OpenAPI 3.1 file, `api/openapi.yaml`, is the machine-readable contract. TypeScript types for the frontend are **generated** from it (`npm run gen:api`), and a test fails if the Go handlers and the OpenAPI file disagree on routes.

## 1. Conventions

**Content.** Request and response bodies are JSON (`application/json; charset=utf-8`), except file uploads and downloads (raw bytes) and SSE streams.

**Names.** JSON fields are `snake_case`. Timestamps are RFC 3339 strings in UTC (`"2026-10-30T09:15:00Z"`).

**Pagination.** List endpoints accept `?page=1&per_page=30` (max 100) and return:

```
Link: </api/v1/repos/alice/demo/issues?page=2&per_page=30>; rel="next", <…page=7…>; rel="last"
X-Total-Count: 193
```

**Errors.** Every error has the same shape:

```json
{
  "error": {
    "code": "not_found",
    "message": "Repository alice/demo not found",
    "details": {}
  },
  "request_id": "01JBX3K9…"
}
```

| HTTP | `code` | Meaning |
|---|---|---|
| 400 | `bad_request` | Malformed JSON or parameters |
| 401 | `unauthenticated` | No or invalid credentials |
| 403 | `forbidden` | Authenticated but not allowed |
| 404 | `not_found` | Missing, **or private and the caller has no read access** (we never reveal that a private repo exists) |
| 409 | `conflict` | State conflict: name taken, merge conflict, base branch moved |
| 422 | `validation_failed` | `details.fields` maps field → message |
| 429 | `rate_limited` | `Retry-After` header set |
| 500 | `internal` | Bug. Logged with `request_id` |

Domain errors in services map to these codes in exactly one place: `internal/api/errors.go`.

**Request IDs.** Every response carries `X-Request-ID`. It appears in every log line for that request, so a user's bug report can be matched to server logs.

**Rate limits.** Per IP for unauthenticated requests (default 60/min) and per user for authenticated ones (default 5000/hour). Login and Git Basic-auth failures have a stricter limit (10/min per IP).

**Versioning.** Breaking changes need a new prefix (`/api/v2`). Adding fields is not breaking. Clients must ignore unknown fields.

## 2. Endpoints

`{o}/{r}` means `{owner}/{repo}`. Auth column: **—** public (for public repos), **U** signed in, **R/W/A** repo read/write/admin, **SA** site admin.

### 2.1 Auth and session

| Method | Path | Auth | Description |
|---|---|---|---|
| POST | `/api/v1/auth/login` | — | `{username, password}` → sets session cookie, returns user + CSRF token |
| POST | `/api/v1/auth/logout` | U | Deletes session |
| POST | `/api/v1/auth/register` | — | Only if `auth.allow_registration = true` |
| GET | `/api/v1/user` | U | Current user |
| PATCH | `/api/v1/user` | U | Update profile |
| PUT | `/api/v1/user/password` | U | `{current, new}` — invalidates other sessions |
| GET/POST | `/api/v1/user/tokens` | U | List / create PAT (the plaintext token is returned **once**) |
| DELETE | `/api/v1/user/tokens/{id}` | U | Revoke |
| GET/DELETE | `/api/v1/user/sessions[/{id}]` | U | List / revoke sessions |

### 2.2 Users

| Method | Path | Auth | Description |
|---|---|---|---|
| GET | `/api/v1/users/{username}` | — | Public profile |
| GET | `/api/v1/users/{username}/repos` | — | Repos visible to caller |

### 2.3 Repositories

| Method | Path | Auth | Description |
|---|---|---|---|
| POST | `/api/v1/repos` | U | `{name, description, visibility, default_branch, init_readme}` |
| GET | `/api/v1/repos/{o}/{r}` | R | Repo details, clone URLs, permissions of caller |
| PATCH | `/api/v1/repos/{o}/{r}` | A | Rename, description, visibility, default branch |
| DELETE | `/api/v1/repos/{o}/{r}` | A | Body must contain `{confirm: "<owner>/<name>"}` |
| POST | `/api/v1/repos/{o}/{r}/forks` | U | Fork into caller's namespace |
| GET/PUT/DELETE | `/api/v1/repos/{o}/{r}/collaborators[/{username}]` | A | `{role}` |
| GET/POST/PATCH/DELETE | `/api/v1/repos/{o}/{r}/protected-branches[/{id}]` | A | |

### 2.4 Repository content

| Method | Path | Auth | Description |
|---|---|---|---|
| GET | `/api/v1/repos/{o}/{r}/branches` | R | Name, head sha, protected flag |
| POST | `/api/v1/repos/{o}/{r}/branches` | W | `{name, from}` |
| DELETE | `/api/v1/repos/{o}/{r}/branches/{name}` | W | Respects protection |
| GET | `/api/v1/repos/{o}/{r}/tags` | R | |
| GET | `/api/v1/repos/{o}/{r}/tree/{ref}/{path...}` | R | Directory listing with last commit per entry |
| GET | `/api/v1/repos/{o}/{r}/blob/{ref}/{path...}` | R | File metadata + content (text ≤ 1 MiB, base64 for binary) |
| GET | `/api/v1/repos/{o}/{r}/raw/{ref}/{path...}` | R | Raw bytes. Served with `Content-Disposition: attachment` and `X-Content-Type-Options: nosniff` |
| GET | `/api/v1/repos/{o}/{r}/commits?ref=&path=` | R | Paginated log |
| GET | `/api/v1/repos/{o}/{r}/commits/{sha}` | R | Commit + diff (+ pipeline status) |
| GET | `/api/v1/repos/{o}/{r}/compare/{base}...{head}` | R | Commits + diff |
| GET | `/api/v1/repos/{o}/{r}/blame/{ref}/{path...}` | R | |
| GET | `/api/v1/repos/{o}/{r}/archive/{ref}.zip` | R | Streamed `git archive` |
| PUT | `/api/v1/repos/{o}/{r}/contents/{path...}` | W | Create/update one file from the web editor: `{branch, message, content_base64, expected_sha}` |

### 2.5 Issues

| Method | Path | Auth | Description |
|---|---|---|---|
| GET | `/api/v1/repos/{o}/{r}/issues?state=&label=&author=&q=` | R | Excludes PRs unless `type=all` |
| POST | `/api/v1/repos/{o}/{r}/issues` | R | Any reader can open an issue |
| GET/PATCH | `/api/v1/repos/{o}/{r}/issues/{n}` | R / author or W | Title, body, state, labels |
| GET/POST | `/api/v1/repos/{o}/{r}/issues/{n}/comments` | R | |
| PATCH/DELETE | `/api/v1/repos/{o}/{r}/comments/{id}` | author or A | |
| GET/POST/DELETE | `/api/v1/repos/{o}/{r}/labels[/{id}]` | R / W | |

### 2.6 Pull requests

| Method | Path | Auth | Description |
|---|---|---|---|
| GET | `/api/v1/repos/{o}/{r}/pulls?state=` | R | |
| POST | `/api/v1/repos/{o}/{r}/pulls` | R (+ read on head repo) | `{title, body, base, head}`; `head` may be `user:branch` for forks |
| GET/PATCH | `/api/v1/repos/{o}/{r}/pulls/{n}` | R / author or W | Includes `merge_status`, `conflict_files`, `checks` |
| GET | `/api/v1/repos/{o}/{r}/pulls/{n}/commits` | R | |
| GET | `/api/v1/repos/{o}/{r}/pulls/{n}/files` | R | Diff per file |
| POST | `/api/v1/repos/{o}/{r}/pulls/{n}/reviews` | R | `{event: comment\|approve\|request_changes, body, comments:[{path,line,side,body}]}` |
| POST | `/api/v1/repos/{o}/{r}/pulls/{n}/merge` | W | `{strategy: merge\|squash, message, expected_head_sha}` → 409 on conflict or if head moved |

### 2.7 CI/CD

| Method | Path | Auth | Description |
|---|---|---|---|
| GET | `/api/v1/repos/{o}/{r}/runs?ref=&status=&event=` | R | |
| POST | `/api/v1/repos/{o}/{r}/runs` | W | Manual run: `{ref, file, inputs}` |
| GET | `/api/v1/repos/{o}/{r}/runs/{id}` | R | Run + jobs graph |
| POST | `/api/v1/repos/{o}/{r}/runs/{id}/cancel` | W | |
| POST | `/api/v1/repos/{o}/{r}/runs/{id}/rerun` | W | `{failed_only: bool}` |
| GET | `/api/v1/repos/{o}/{r}/jobs/{id}` | R | Job + steps |
| GET | `/api/v1/repos/{o}/{r}/jobs/{id}/log?offset=` | R | Plain text from byte offset |
| GET | `/api/v1/repos/{o}/{r}/jobs/{id}/log/stream` | R | **SSE**: `event: chunk` `{offset, text}`, `event: step` `{index,status}`, `event: done` |
| GET | `/api/v1/repos/{o}/{r}/runs/{id}/artifacts` | R | |
| GET | `/api/v1/repos/{o}/{r}/artifacts/{id}/download` | R | |
| GET/PUT/DELETE | `/api/v1/repos/{o}/{r}/secrets[/{NAME}]` | A | Values are write-only and never returned |
| GET | `/api/v1/repos/{o}/{r}/commits/{sha}/status` | R | Combined status of all runs for a commit |

### 2.8 Runners (management)

| Method | Path | Auth | Description |
|---|---|---|---|
| GET | `/api/v1/admin/runners` | SA | All runners |
| POST | `/api/v1/admin/runners/registration-tokens` | SA | `{max_uses, expires_in}` → token returned once |
| PATCH/DELETE | `/api/v1/admin/runners/{id}` | SA | Labels, disable, remove |
| GET/POST | `/api/v1/repos/{o}/{r}/runners[/registration-tokens]` | A | Repo-scoped runners |

### 2.9 Releases, packages, deployments

| Method | Path | Auth | Description |
|---|---|---|---|
| GET | `/api/v1/repos/{o}/{r}/releases` | R | |
| POST | `/api/v1/repos/{o}/{r}/releases` | W / job token | `{tag_name, target, title, notes, draft, prerelease}` — creates the tag if missing |
| GET/PATCH/DELETE | `/api/v1/repos/{o}/{r}/releases/{id}` | R / W | |
| POST | `/api/v1/repos/{o}/{r}/releases/{id}/assets?name=` | W / job token | Raw body upload |
| GET | `/api/v1/repos/{o}/{r}/releases/{id}/assets/{name}` | R | Download |
| GET | `/api/v1/repos/{o}/{r}/releases/latest` | R | Newest non-draft, non-prerelease release |
| GET | `/api/v1/repos/{o}/{r}/releases/tags/{tag}` | R | Release by tag name |
| GET | `/api/v1/repos/{o}/{r}/releases/latest/assets/{name}` | R | Download an asset of the latest release (stable URL for deploy scripts) |
| PUT | `/api/v1/repos/{o}/{r}/packages/generic/{name}/{version}/{file}` | W / job token / PAT `package:write` | Raw body. 409 if the file already exists |
| GET | `/api/v1/repos/{o}/{r}/packages/generic/{name}/{version}/{file}` | R | Download |
| GET | `/api/v1/repos/{o}/{r}/packages` | R | |
| GET | `/api/v1/repos/{o}/{r}/deployments?environment=` | R | |

### 2.10 Events and the Architecture Explorer

| Method | Path | Auth | Description |
|---|---|---|---|
| GET | `/api/v1/repos/{o}/{r}/activity` | R | Activity feed from events |
| GET | `/api/v1/explorer/topology` | U | Live component list: version, uptime, DB size, repo count, queue depth, runners online |
| GET | `/api/v1/explorer/stream` | U | **SSE** of trace events (`git.request`, `hook.call`, `task.started`, `job.claimed`, …), filtered to repos the user can read; admins see everything |
| GET | `/api/v1/explorer/traces/{trace_id}` | U | All recorded spans of one trace (for replay) |

### 2.11 Admin and system

| Method | Path | Auth | Description |
|---|---|---|---|
| GET/POST/PATCH | `/api/v1/admin/users[/{username}]` | SA | Create, deactivate, promote |
| GET/PATCH | `/api/v1/admin/settings` | SA | |
| GET | `/api/v1/admin/tasks?status=` | SA | Queue inspection |
| GET | `/healthz` | — | Liveness: `200 ok` |
| GET | `/readyz` | — | Readiness: DB reachable, git available, migrations current |
| GET | `/api/v1/version` | — | `{version, commit, build_date, go_version, git_version}` |

## 3. Internal API (hooks)

Bound to `127.0.0.1:<internal_port>` only. Every request needs `Authorization: Bearer <ANVIL_INTERNAL_TOKEN>`.

| Method | Path | Body | Response |
|---|---|---|---|
| POST | `/internal/hooks/pre-receive` | `{repo_id, pusher_id, push_id, updates:[{old,new,ref}]}` | `{allowed, message}` |
| POST | `/internal/hooks/post-receive` | same | `{messages:[…]}` |

## 4. Example: create a PAT and clone

```bash
# 1. Log in (browser does this) — or create the token in Settings → Tokens.
curl -s -X POST https://anvil.local/api/v1/user/tokens \
  -H "Authorization: Bearer $EXISTING_TOKEN" \
  -d '{"name":"laptop","scopes":["repo:write"]}'
# → {"id":7,"token":"anv_pat_3fX9…","scopes":["repo:write"]}

# 2. Clone with it.
git clone https://alice:anv_pat_3fX9…@anvil.local/alice/demo.git
```
