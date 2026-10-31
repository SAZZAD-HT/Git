# Changelog

All notable user-visible changes. Format: [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).

## Unreleased

### Added (P4 Collaboration)

- Issues: open, edit, close and reopen, comment with Markdown (write and preview tabs), edit or delete your comments, and filter the list by state, label, author or text.
- Labels: writers create, rename, recolour and delete labels and attach them to issues and pull requests.
- Pull requests: "Create pull request" on the compare page; Conversation, Commits and Files changed tabs; the branch's new pushes are picked up automatically.
- Reviews: comment on individual diff lines, then submit the review as a comment, approval or change request. Authors cannot approve their own pull request.
- Merging: the merge box shows whether the branch merges cleanly (and which files conflict), and writers merge with a merge commit or squash. If someone pushes while you are looking, the merge is refused instead of merging commits you have not seen.
- Issues and pull requests share one number sequence per repository.

### Added (P3 Git hosting, in progress)

- Create repositories (public or private, optional README), rename, change default branch, delete with confirmation.
- `git clone`, `fetch` and `push` over HTTP(S) with a personal access token as the password; protocol v2 and gzip supported.
- Server-side hooks: pushing a new branch prints a "Create a pull request" link; the first push to an empty repository sets its default branch.
- Branch protection (exact names or globs such as `release/*`): blocks force-pushes, deletion, and optionally all direct pushes. The default branch cannot be deleted; `refs/pull/*` is read-only.
- Collaborators with read, write or admin roles. Private repositories return 404 to anyone without access.
- API for browsing: branches, tags, directory listings, files, raw downloads, commit history.
- `api/openapi.yaml`: OpenAPI 3.1 contract for every REST endpoint, kept in sync with the router by a test.

### Added (P2 Identity)

- Sign in and out with username and password; browser sessions with CSRF protection (14-day sliding, 30-day absolute expiry).
- Self-registration, off by default (`auth.allow_registration`).
- Personal access tokens with scopes (`repo:read`, `repo:write`, `package:write`, `admin`) and optional expiry, usable as Bearer tokens or as the password for git over HTTP.
- Change password (ends your other sessions), list and revoke sessions.
- Site admin: list, create, promote, demote, activate and deactivate users, from the API or with `anvil admin`.
- Rate limits: 60 requests a minute per IP when anonymous, 5000 an hour per user, 10 failed sign-ins a minute per IP and per username.
- Passwords: at least 10 characters, not among the 10,000 most common, stored as argon2id.

### Added (P1 Foundation)

- `anvil init`: writes a commented `anvil.toml` with a random secret key (owner-only permissions) and creates the data directory.
- `anvil serve`: starts the server, applies database migrations automatically, serves `/healthz`, `/readyz` and `/api/v1/version`, and shuts down gracefully on Ctrl+C, SIGTERM or service stop.
- `anvil migrate [--dry-run]`, `anvil doctor`, `anvil version`.
- `anvil install-service` / `uninstall-service`: systemd unit on Linux, Windows service (no NSSM) on Windows.
- Every config key can be overridden with `ANVIL_<SECTION>_<KEY>`.
- Structured logs with `request_id` and `trace_id` on every request line.
- `go run ./tools/devtool build|test|lint|dist|dev|docs`.
