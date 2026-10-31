# Progress and handoff

Where the build of Anvil stands, so a new session on any machine (local, remote or claude.ai/code cloud) can continue. **Update this file at the end of every work session** in the same commit as the work.

Last updated: 2026-10-31, after the P4 web pages (branch `p4-web`, commit `9679afa`).

## 1. Phase status

Phases come from `docs/04-SPMP-Software-Project-Management-Plan.md` §3.

| Phase | Status | Notes |
|---|---|---|
| P0 Requirements & design | Done | SRS, SDD, STP, SPMP, RTM in `docs/` |
| P1 Foundation | Done | `init`, `serve`, migrations, logging, Windows service / systemd, devtool, CI |
| P2 Identity & access | Done | Sessions + CSRF, PATs with scopes, rate limits, admin users, settings pages |
| P3 Git hosting & browsing | Mostly done | See open items in §3 |
| P4 Issues & pull requests | Done except forks and required checks | Backend merged to `main` (PR #1). Web pages on `p4-web` (not merged yet) |
| P5 CI engine & runner | **Next** | Design: `docs/reference/07-ci-cd-design.md` |
| P6 Publishing | Not started | `docs/reference/08-publishing.md` |
| P7 Architecture Explorer | Not started | |
| P8 Hardening & release | Not started | |

## 2. Branches on GitHub (`SAZZAD-HT/Git`)

| Branch | State |
|---|---|
| `main` | P1–P4 backend (merged PR #1) |
| `p4-web` | `main` + P4 web pages. **Open a PR and merge before starting P5.** |
| `p4-collaboration`, `p1-foundation`, `linux-verification`, `openapi-spec` | Already merged; can be deleted |

Start new work from `main` after `p4-web` is merged: `git checkout main && git pull && git checkout -b p5-ci`.

## 3. Open items (not blocking P5)

- **P3:** forks; pooled `git cat-file --batch`; blame view; web file editor (PutFile); scheduled `git gc` maintenance.
- **P4:** pull requests from forks (needs forks); required status checks before merge (FR-COLLAB-10) need P5 check runs.
- **Code cleanups found in review:** `cmd/anvil/admin.go` branches on `runtime.GOOS` instead of `_windows.go`/`_unix.go` files; `clone_url` always uses `server.public_url`.
- **Not yet verified:** `install-service` as a real systemd unit; arm64 builds at runtime.
- **P4 UI:** checked by typecheck, unit tests and an API walk-through, but no one has clicked through it in a browser yet.

## 4. Next step: P5 plan

Read `docs/reference/07-ci-cd-design.md`, `docs/reference/14-code-structure.md` §2 (where `internal/ci` and `cmd/anvil-runner` sit) and the TC-CI-* cases in `docs/03-STP-Software-Test-Plan.md`. Suggested order:

1. Migration `0005_ci.sql`: runners, runs, jobs, steps, logs, artifacts, check runs.
2. IR types plus parsers for `.github/workflows/*.yml` and `.gitlab-ci.yml`, with table tests.
3. Trigger on the `git.push` and `pull.*` events, creating runs and jobs.
4. Scheduler: job DAG, runner labels, atomic claim (claim race test ×1000).
5. Runner API (register, long-poll, logs, artifacts), then `cmd/anvil-runner` running PowerShell/bash steps.
6. Web pages: pipelines list, run page with live logs, runner admin.
7. Required checks in branch protection and the PR merge box (closes FR-COLLAB-10).
8. Exit test: Anvil builds itself on Windows and Linux.

## 5. Setting up a new machine

Needs **Go 1.27** (version from `go.mod`), **Node 22+**, **git 2.40+**. No Docker.

```sh
git clone https://github.com/SAZZAD-HT/Git.git anvil && cd anvil
cd web && npm ci && cd ..
go run ./tools/devtool build    # web + bin/anvil(.exe)
go run ./tools/devtool test     # Go tests + web vitest
go run ./tools/devtool lint     # gofmt, go vet (both OSes)
go run ./tools/devtool dev      # local instance in ./.dev on :3000
```

Create a user for the dev instance: `ANVIL_CONFIG=.dev/anvil.toml ./bin/anvil admin create-user -username you -email you@example.com -admin -password-stdin`. Passwords need 10+ characters and must not contain the username.

## 6. Things that trip sessions up

- Always run `devtool lint` before pushing, and stop if it fails (gofmt import order has broken CI before).
- `internal/archtest` enforces the layer rules: only `internal/store` imports `database/sql`; service packages must not import `net/http`; every package needs `doc.go`.
- New API routes go in `internal/api` **and** `api/openapi.yaml`. A test checks they match. Repo sub-routes mount inside the `/repos/{owner}/{repo}` route in `internal/api/repos.go`. Mounting a second router on that path panics at startup, and `internal/httpserver/routes_test.go` catches it.
- Windows Smart App Control may block freshly built test binaries. Re-run them, or rely on Linux CI. Don't try to bypass it.
- In bash, don't put TypeScript or Go containing backticks inside `node -e "…"`. Use file edits instead.
- Error responses go through `internal/api/errors.go` only; services return `errs.*` kinds.
