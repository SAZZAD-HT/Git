# 02 — Technology Decisions

Each decision is written as an **ADR** (Architecture Decision Record): context, options, decision, consequences. ADRs are never edited after acceptance. If a decision changes, add a new ADR that *supersedes* the old one and update its status.

## Summary

| Part | Choice |
|---|---|
| Server backend (the "Gitea rewrite") | **Go** |
| CI runner | **Go** (same repository, second binary) |
| Frontend | **TypeScript + React**, built with **Vite** |
| Database | **SQLite** (default, embedded) with a path to **PostgreSQL** |
| Git engine | The **`git` CLI** (≥ 2.38), wrapped by `internal/git` |
| Pipeline file parsing | YAML via `go.yaml.in/yaml/v3` |
| Live updates to the browser | **Server-Sent Events (SSE)** |
| Architecture diagrams in the UI | **React Flow** (`@xyflow/react`) |
| Docs and diagrams | Markdown + Mermaid in `docs/` |

---

## ADR-001 — Backend language: Go

**Status:** Accepted

**Context.** The server must ship as a single executable for Windows and Linux (constraint C3), stream large Git packs efficiently, run many concurrent long-polls from runners, and be readable by new developers. The two reference implementations we study most (Gitea and gitlab-runner) are written in Go.

**Options compared** (5 = best):

| Criterion | Go | Rust | TypeScript / Node.js | C# / .NET | Java / Kotlin | Python |
|---|:-:|:-:|:-:|:-:|:-:|:-:|
| Single self-contained binary for Win + Linux | 5 | 5 | 2 | 4 | 2 | 1 |
| Cross-compiling from one machine | 5 | 3 | 3 | 4 | 3 | 1 |
| Concurrency (thousands of streams and long-polls) | 5 | 5 | 4 | 5 | 4 | 2 |
| Learning curve for new developers | 4 | 2 | 5 | 4 | 3 | 5 |
| Process control (spawning git, killing process trees) | 5 | 5 | 3 | 4 | 3 | 3 |
| Existing forge/runner code to learn from | 5 (Gitea, Gogs, gitlab-runner, act_runner) | 1 | 1 | 2 (Bonobo, actions/runner) | 1 | 2 (Pagure) |
| Memory footprint | 5 | 5 | 3 | 3 | 2 | 3 |
| Build speed / tooling simplicity | 5 | 2 | 4 | 4 | 3 | 4 |
| **Total** | **39** | 28 | 25 | 30 | 21 | 21 |

**Decision.** Go.

**Consequences.**
- (+) `GOOS=windows go build` and `GOOS=linux go build` produce release binaries from any machine.
- (+) Standard library covers HTTP server, TLS, process exec, gzip, `embed`, and structured logging (`log/slog`).
- (+) New developers can read Gitea's source as a side-by-side reference.
- (−) Developers must install Go (one installer, no other setup).
- (−) Frontend and backend use different languages. Mitigated by a typed API contract ([06](06-api-spec.md)) and generated TypeScript types.

## ADR-002 — Frontend: TypeScript + React + Vite

**Status:** Accepted

**Context.** We need a rich UI (file browser, diff viewer, live job logs, an interactive architecture diagram) served from the Go binary.

**Options:** React, Vue 3, Svelte, server-rendered Go templates (like Gitea), HTMX.

**Decision.** React with TypeScript, built by Vite into static files that the Go binary embeds with `//go:embed`.

**Why.**
- React has the largest ecosystem for what we need: React Flow for diagrams, mature diff and code-view components, TanStack Query for server state.
- TypeScript catches API contract mismatches at compile time.
- Most frontend developers already know React, which shortens onboarding.
- Server-rendered templates were rejected because the Explorer and live logs need a stateful client anyway, and two UI systems would double the learning load.

**Main libraries:**

| Purpose | Library |
|---|---|
| Routing | React Router |
| Server state, caching | TanStack Query |
| Diagrams (Architecture Explorer, pipeline graph) | React Flow (`@xyflow/react`) |
| Syntax highlighting | Shiki |
| Markdown | `marked` + **DOMPurify** (mandatory sanitising, see [10](10-security.md)) |
| Styling | CSS Modules + CSS custom properties (design tokens), light and dark themes |
| Tests | Vitest + Testing Library; Playwright for end-to-end |

## ADR-003 — CI runner in Go, same repository

**Status:** Accepted

**Context.** The runner must run on every machine that executes jobs, often a different machine from the server, and must kill whole process trees on cancel (Windows Job Objects, Unix process groups).

**Decision.** `cmd/anvil-runner`, a second Go binary in the same module. It shares the pipeline model and the API client types with the server, so the protocol cannot drift between them.

## ADR-004 — Git engine: the `git` command line, not a library

**Status:** Accepted

**Context.** Options: (a) call the `git` program, (b) go-git (pure Go), (c) libgit2 via cgo.

**Decision.** (a) The `git` CLI, version 2.38 or newer.

**Why.** The server *is* standard Git for `upload-pack` and `receive-pack`, so protocol behaviour is exactly what clients expect, including protocol v2 and partial clone. It is fast, battle-tested, and the approach Gitea and GitLab (inside Gitaly) take. cgo (libgit2) would break easy cross-compilation.

**Consequences.** Git must be installed on the server and on runners. On Windows, Git for Windows also provides `sh.exe`, which runs our hook scripts ([12](12-cross-platform.md)). All calls go through `internal/git/gitcmd`, which enforces timeouts and protects against argument injection ([10](10-security.md)).

## ADR-005 — Database: SQLite by default, PostgreSQL-ready

**Status:** Accepted

**Decision.** SQLite via **`modernc.org/sqlite`**, a pure-Go driver that needs no cgo, so cross-compiling still works. Run in WAL mode with a single writer connection plus a read pool. Queries are written in portable SQL through a thin `internal/store` layer, so PostgreSQL (via `pgx`) can be added later without touching the service code.

**Why not an ORM.** Hand-written SQL in one package is easier for a new developer to read and debug than ORM-generated queries, and every query is documented in place.

**Migrations.** Numbered SQL files embedded in the binary (`internal/store/migrations/0001_init.sql`, …). They are applied automatically at startup inside a transaction and recorded in a `schema_migrations` table.

## ADR-006 — One pipeline model, two file formats

**Status:** Accepted

**Decision.** `.github/workflows/*.yml` and `.gitlab-ci.yml` are parsed by two separate front-end parsers into one internal model (the **IR**, `internal/ci/model`). The scheduler, runner and UI only ever see the IR. Supporting a third format later means writing one new parser.

## ADR-007 — Live updates over Server-Sent Events

**Status:** Accepted

**Options:** polling, WebSockets, SSE.

**Decision.** SSE (`text/event-stream`) for job logs, pipeline status and the Explorer event stream.

**Why.** All live traffic flows from server to browser only. SSE is plain HTTP, so it passes proxies, reconnects automatically with `Last-Event-ID`, and needs no extra library in Go or the browser.

## ADR-008 — Architecture style: modular monolith

**Status:** Accepted

**Decision.** One server process split into packages with strict dependency rules ([03](03-architecture-overview.md), [14](14-code-structure.md)). Packages talk to each other through Go interfaces and the in-process event bus, never by reaching into each other's tables.

**Why.** It is as easy to deploy as Gitea, and the module boundaries mean any module (for example CI scheduling) could later be split into a separate service, as GitLab did with Gitaly.

## ADR-009 — HTTP router: `chi`

**Status:** Accepted

**Decision.** `github.com/go-chi/chi/v5`. It uses standard `net/http` handlers, has route groups for middleware, has no hidden behaviour, and is small enough to read in an afternoon.

## ADR-010 — Configuration: TOML file + environment overrides

**Status:** Accepted

**Decision.** `anvil.toml` (parsed with `github.com/BurntSushi/toml`). Every key can be overridden by an environment variable `ANVIL_<SECTION>_<KEY>`. TOML is easier to hand-edit correctly than YAML and has no surprising type coercion. Full reference in [11](11-configuration-and-operations.md).

## Dependency policy

- Prefer the Go standard library. Each new third-party dependency needs a one-line justification in the pull request and must have a permissive license (C4).
- Pin exact versions (`go.sum`, `package-lock.json`). Dependabot-style update checks run weekly.
- No dependency may require cgo or a system library other than `git`.

## Toolchain versions (initial)

| Tool | Version | Used for |
|---|---|---|
| Go | latest stable at project start (≥ 1.25) | server, runner |
| Node.js | 24 LTS | building the frontend only (not needed at runtime) |
| Git | ≥ 2.38 (dev machine has 2.53) | runtime dependency of server and runner |
| golangci-lint | latest | Go linting, including doc-comment rules |
