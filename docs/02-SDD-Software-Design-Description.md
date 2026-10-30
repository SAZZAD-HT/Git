# Software Design Description (SDD)

**Project:** Anvil — Self-Hosted Code Forge and CI/CD Platform
**Document ID:** ANV-SDD-001 · **Version:** 1.0 (Draft for approval) · **Date:** 2026-10-30
**Standard:** IEEE 1016-2009 (Software Design Descriptions)

## Document Control

| Version | Date | Author | Change |
|---|---|---|---|
| 1.0 | 2026-10-30 | Project team | First complete design description, organised by IEEE 1016 viewpoints |

| Role | Name | Signature / Date |
|---|---|---|
| Technical Lead | | |
| Product Owner | SAZZAD-HT | |

---

## Table of Contents

1. [Introduction](#1-introduction)
2. [Design Stakeholders and Concerns](#2-design-stakeholders-and-concerns)
3. [Context Viewpoint](#3-context-viewpoint)
4. [Composition Viewpoint](#4-composition-viewpoint)
5. [Logical Viewpoint](#5-logical-viewpoint)
6. [Dependency Viewpoint](#6-dependency-viewpoint)
7. [Information Viewpoint](#7-information-viewpoint)
8. [Interface Viewpoint](#8-interface-viewpoint)
9. [Interaction Viewpoint](#9-interaction-viewpoint)
10. [State Dynamics Viewpoint](#10-state-dynamics-viewpoint)
11. [Algorithm Viewpoint](#11-algorithm-viewpoint)
12. [Resource and Deployment Viewpoint](#12-resource-and-deployment-viewpoint)
13. [Concurrency Viewpoint](#13-concurrency-viewpoint)
14. [Security Viewpoint](#14-security-viewpoint)
15. [Portability Viewpoint](#15-portability-viewpoint)
16. [Design Rationale](#16-design-rationale)
17. [Module Design Specifications](#17-module-design-specifications)

---

## 1. Introduction

### 1.1 Purpose

This SDD describes **how** Anvil satisfies the requirements in [ANV-SRS-001](01-SRS-Software-Requirements-Specification.md). Developers implement from it, reviewers check changes against it, and new team members learn the system from it.

### 1.2 Scope

It covers the `anvil` server, the `anvil-runner` program, and the React web client, for release 1.0.

### 1.3 Design Overview

Anvil is a **modular monolith**: one Go executable containing layered modules (transport → services → infrastructure), plus a separate runner executable. Git storage is delegated to the `git` program; metadata lives in embedded SQLite; the web UI is a React single-page application embedded in the server binary.

### 1.4 References

[ANV-SRS-001](01-SRS-Software-Requirements-Specification.md) · [ANV-STP-001](03-STP-Software-Test-Plan.md) · [Diagram index](diagrams/README.md) · Technical annexes in [reference/](reference/README.md) (detailed protocol, API and schema specifications).

---

## 2. Design Stakeholders and Concerns

| Stakeholder | Concerns | Addressed in |
|---|---|---|
| Product Owner | Feature coverage, Windows + Linux support, no Docker | §3, §12, §15 |
| Developers | Module boundaries, interfaces, where code goes | §4–§6, §17 |
| Testers | Observable behaviour, states, error cases | §9–§11 |
| Operators | Installation, resources, backup | §12 |
| Security reviewers | Authentication, isolation, secrets | §14 |
| New team members | Understanding the system end to end | All, plus [reference/13-onboarding.md](reference/13-onboarding.md) |

---

## 3. Context Viewpoint

![CX-01](diagrams/png/CX-01-system-context.png)

*Figure 3-1 — System context*

- All connections are **inbound** to Anvil over HTTP(S). Runners initiate every exchange, so runners can sit behind NAT/firewalls.
- Deploy targets are machines running `anvil-runner` with deployment labels; there is no separate deploy protocol.

---

## 4. Composition Viewpoint

### 4.1 Server components

![CP-01](diagrams/png/CP-01-server-components.png)

*Figure 4-1 — `anvil` server components (UML component diagram)*

| Layer | Component | Responsibility | SRS |
|---|---|---|---|
| Transport | `httpserver` | Listener, middleware chain (request ID, recovery, logging, auth, CSRF, rate limit, trace, security headers), SPA serving | IR-1, NFR-SEC-02/05 |
| Transport | `api` | REST handlers `/api/v1` — decode, call one service, encode | IR-2 |
| Transport | `git/smarthttp` | Git smart HTTP endpoints | IR-3, FR-REPO-04/05 |
| Transport | `ci/runnerapi` | Runner protocol endpoints | IR-4 |
| Transport | `git/hooks` | Hook scripts, `anvil hook` client, internal API | IR-5, FR-REPO-06/07 |
| Service | `auth` | Credentials, sessions, tokens, permission evaluation | FR-AUTH-* |
| Service | `users` | User accounts | FR-AUTH-03/09, FR-ADM-01 |
| Service | `repos` | Repository lifecycle, content browsing, protection, push checks | FR-REPO-* |
| Service | `issues` | Issues, comments, labels | FR-COLLAB-01…04 |
| Service | `pulls` | Pull requests, mergeability, review, merge | FR-COLLAB-05…11 |
| Service | `ci/trigger` | Decide which pipelines run for an event | FR-CI-01…07 |
| Service | `ci/scheduler` | Job states, claiming, DAG advancement, timeouts | FR-CI-08…12 |
| Service | `artifacts`, `releases`, `packages`, `deployments` | Publishing | FR-PUB-* |
| Service | `secrets` | Encrypted CI secrets | FR-CI-13/14 |
| Service | `explorer` | Trace spans, topology, Explorer API | FR-EXP-* |
| Infrastructure | `store` | All SQL and migrations | DR-1/2 |
| Infrastructure | `git`, `git/gitcmd` | All repository disk access through the `git` executable | CON-6, NFR-SEC-03 |
| Infrastructure | `blob` | Content-addressed file storage | DR-5 |
| Infrastructure | `events` | In-process event bus + event log | DR-4 |
| Infrastructure | `queue` | Durable background task queue | NFR-REL-01 |
| Infrastructure | `ci/model`, `ci/parser/*`, `ci/expr`, `ci/logs` | Pipeline IR, parsers, expression engine, job log storage | FR-CI-01…04, FR-CI-09 |

### 4.2 Runner components

![CP-02](diagrams/png/CP-02-runner-components.png)

*Figure 4-2 — `anvil-runner` components*

| Component | Responsibility | SRS |
|---|---|---|
| `poller` | One long-poll loop per capacity slot | FR-CI-08 |
| `client` | Typed runner API client with retries | IR-4, NFR-REL-06 |
| `executor` | Runs one job end to end | FR-CI-17 |
| `workspace` | Per-job directories and cleanup | NFR-SEC-06 |
| `checkout` | Fetches exact commit with per-command token | FR-CI-21, NFR-SEC-06 |
| `shell` | Detects shells, renders step scripts | FR-CI-17 |
| `procgroup` | Starts and kills process trees (Job Object / process group) | FR-CI-18 |
| `masker` | Masks secrets in output | FR-CI-15 |
| `actions` | Built-in `uses:` implementations | FR-CI-21 |

### 4.3 Web client

React + TypeScript SPA, structured by feature (`auth`, `repo`, `issues`, `pulls`, `ci`, `releases`, `packages`, `admin`, `explorer`). Server state is cached with TanStack Query and updated by SSE. Details: [reference/09-frontend-architecture.md](reference/09-frontend-architecture.md).

---

## 5. Logical Viewpoint

### 5.1 Domain model

![CD-01](diagrams/png/CD-01-domain-model.png)

*Figure 5-1 — Domain model (UML class diagram)*

Key rules shown by the model:

- An `Issue` with `isPull = true` has exactly one `PullRequest`; issues and PRs share the repository's number sequence.
- A `Run` is composed of one or more `Job`s; a `Job` of `JobStep`s. A `Job` is executed by at most one `Runner`.
- Releases, packages, deployments and secrets are owned by (composed into) a `Repository` and are deleted with it.

### 5.2 Git hosting subsystem

![CD-02](diagrams/png/CD-02-git-subsystem.png)

*Figure 5-2 — Git hosting classes*

`smarthttp.Handler` authenticates and authorises, then spawns `git` through `gitcmd.Cmd`. `git.Repository` wraps read operations and keeps a pool of `git cat-file --batch` processes. `hooks` connects Git's server hooks back to `repos.Service` through the internal API.

### 5.3 CI/CD engine (server)

![CD-03](diagrams/png/CD-03-ci-subsystem.png)

*Figure 5-3 — CI/CD engine classes*

Two `Parser` implementations (GitHub, GitLab) produce the same `Pipeline` IR. `TriggerService` evaluates events into runs; `SchedulerService` owns every job state transition; `RunnerAPI` exposes it to runners; `LogStore` persists and streams logs.

### 5.4 Runner

![CD-04](diagrams/png/CD-04-runner.png)

*Figure 5-4 — Runner classes*

`Group` has OS-specific realisations: `GroupWindows` (Job Object) and `GroupUnix` (process group).

### 5.5 Security model

![CD-05](diagrams/png/CD-05-security.png)

*Figure 5-5 — Authentication, authorisation and secrets*

Every request is resolved to an `Actor`. `Authorizer.Permission` is the **single** place that computes repository access.

---

## 6. Dependency Viewpoint

| From layer | May depend on | Must not depend on |
|---|---|---|
| Transport | Services, Infrastructure | — |
| Services | Infrastructure; other services **through interfaces** | Transport, `net/http` |
| Infrastructure | Base (`config`, `logging`, `version`, `errs`, `fsutil`) | Services, Transport |
| Runner | `ci/model`, `ci/expr`, `fsutil`, `version` | Any other server package |

Exclusive ownership rules:

| Resource | Only accessed by |
|---|---|
| SQL database | `internal/store` |
| Repository directories | `internal/git` (via `gitcmd`) |
| Data directory outside `repos/` | `internal/blob`, `internal/ci/logs` |
| `os/exec` | `gitcmd`, runner `procgroup`/`shell`/`checkout`, `service` |

These rules are checked automatically by `internal/archtest` (NFR-MAINT-02). Package-by-package detail: [reference/14-code-structure.md §2](reference/14-code-structure.md).

---

## 7. Information Viewpoint

### 7.1 Database

![ER-01](diagrams/png/ER-01-database.png)

*Figure 7-1 — Database entity-relationship diagram*

Conventions: integer IDs; UTC millisecond timestamps; enums as checked text; tokens stored as SHA-256; secrets as AES-256-GCM ciphertext; foreign keys enforced. Full column-level schema: [reference/05-data-model.md](reference/05-data-model.md).

### 7.2 File storage

```
<data_dir>/
├── anvil.db                       metadata (SQLite, WAL mode)
├── repos/<owner>/<name>.git/      bare Git repositories
├── logs/<run_id>/<job_id>.log     append-only job logs
├── blobs/sha256/<ab>/<cd>/<hash>  artifacts, release assets, packages
├── tmp/                           in-progress uploads
└── secret.key                     master key for secrets
```

### 7.3 Consistency rules

1. Never hold a database transaction while running `git` or doing network I/O.
2. Disk + DB changes: prepare on disk in a temporary location → commit DB → move into place.
3. Job state changes use conditional updates (`… WHERE status = expected`).
4. Ref updates by the server use Git's compare-and-swap (`update-ref <ref> <new> <old>`).

---

## 8. Interface Viewpoint

| Interface | Protocol | Consumers | Authentication | Specification |
|---|---|---|---|---|
| REST API `/api/v1` | HTTPS + JSON, OpenAPI 3.1 | SPA, scripts | Session + CSRF, or bearer PAT / job token | [reference/06-api-spec.md](reference/06-api-spec.md) |
| Git smart HTTP | Git protocol v0/v2 over HTTPS | git clients, runners | HTTP Basic (PAT / job token) | [reference/04-git-hosting.md §2](reference/04-git-hosting.md) |
| Runner API `/api/runner/v1` | HTTPS + JSON, long-poll | anvil-runner | Runner token, job token | [reference/07-ci-cd-design.md §7](reference/07-ci-cd-design.md) |
| Internal API `/internal` | HTTP on 127.0.0.1 | `anvil hook` | Per-process internal token | [reference/06-api-spec.md §3](reference/06-api-spec.md) |
| SSE streams | `text/event-stream` | SPA | Session | [reference/06-api-spec.md §2.7, §2.10](reference/06-api-spec.md) |
| CLI | Command line | Operators | OS user | [reference/11-configuration-and-operations.md §4](reference/11-configuration-and-operations.md) |
| Configuration | TOML file + `ANVIL_*` env vars | Operators | File permissions | [reference/11-configuration-and-operations.md §3](reference/11-configuration-and-operations.md) |

**Error contract:** every API error has the shape `{error: {code, message, details}, request_id}`; domain errors map to HTTP status codes in one place (`internal/api/errors.go`).

---

## 9. Interaction Viewpoint

Each figure is a UML sequence diagram of one key scenario.

### 9.1 Sign in
![SD-01](diagrams/png/SD-01-login.png)

### 9.2 Clone / fetch
![SD-02](diagrams/png/SD-02-clone.png)

### 9.3 Push with server hooks
![SD-03](diagrams/png/SD-03-push.png)

The pusher's identity reaches the hook through environment variables set when `git receive-pack` is spawned; the hook calls the internal API, which applies branch protection (pre-receive) and publishes events (post-receive). The client's push completes before any pipeline work starts.

### 9.4 Open pull request and check mergeability
![SD-04](diagrams/png/SD-04-open-pull-request.png)

### 9.5 Merge pull request
![SD-05](diagrams/png/SD-05-merge-pull-request.png)

### 9.6 Pipeline trigger
![SD-06](diagrams/png/SD-06-pipeline-trigger.png)

### 9.7 Job claim and execution
![SD-07](diagrams/png/SD-07-job-execution.png)

### 9.8 Release and deploy
![SD-08](diagrams/png/SD-08-release-and-deploy.png)

### 9.9 Cancel a running job
![SD-09](diagrams/png/SD-09-cancel-job.png)

---

## 10. State Dynamics Viewpoint

### 10.1 Job
![ST-01](diagrams/png/ST-01-job-states.png)

Only `ci/scheduler` may change a job's state. Run status is derived from job states:

| Condition (in order) | Run status |
|---|---|
| any job running / cancelling | running |
| any job queued / waiting | running if any started, else queued |
| run was cancelled | cancelled |
| any job failed without allow_failure | failure |
| only manual jobs left | blocked |
| otherwise | success |

### 10.2 Pull request
![ST-02](diagrams/png/ST-02-pull-request-states.png)

### 10.3 Runner
![ST-03](diagrams/png/ST-03-runner-states.png)

---

## 11. Algorithm Viewpoint

### 11.1 Push processing
![AD-01](diagrams/png/AD-01-push-processing.png)

### 11.2 Pipeline trigger evaluation
![AD-02](diagrams/png/AD-02-trigger-evaluation.png)

### 11.3 Scheduler advance
![AD-03](diagrams/png/AD-03-scheduler-advance.png)

### 11.4 Runner job execution
![AD-04](diagrams/png/AD-04-runner-job.png)

### 11.5 Merge pull request
![AD-05](diagrams/png/AD-05-merge-pull-request.png)

### 11.6 Key algorithms (pseudocode)

**Job claim — exactly one runner wins (NFR-REL-02)**

```
function Claim(runner):
  repeat until long-poll timeout:
    for job in SELECT queued jobs ORDER BY queued_at LIMIT 50:
      if job.runsOn ⊆ runner.labels:
        rows = UPDATE jobs SET status='running', runner_id=runner.id
               WHERE id = job.id AND status = 'queued'
        if rows == 1: return job          -- we won
    wait on notifier (or timeout)
  return none
```

**Idempotent log append (NFR-REL-06)**

```
function AppendLog(job, offset, data):
  size = job.log_size
  if offset + len(data) <= size: return ok        -- duplicate, already stored
  if offset > size: return Gap(size)               -- runner resends from size
  write data[size - offset:] at position size
  job.log_size = offset + len(data)
```

**Secret masking across chunk boundaries (FR-CI-15)**

```
hold = maxSecretLength - 1
on Write(p):
  buf = pending + p
  replace every secret in buf with "***"
  emit buf[0 : len(buf) - hold]; pending = buf[len(buf) - hold :]
on Flush(): emit masked(pending)
```

**Merge with compare-and-swap (NFR-REL-03)** — see Figure 11-5: `merge-tree` → `commit-tree` → `update-ref base new expectedOld`; a failed swap returns 409 and nothing is changed.

---

## 12. Resource and Deployment Viewpoint

### 12.1 Single machine
![DP-01](diagrams/png/DP-01-single-server.png)

### 12.2 Team / production
![DP-02](diagrams/png/DP-02-team-production.png)

| Resource | Budget / behaviour |
|---|---|
| Server memory | ≤ 1 GB at 50 users + 50 runners (NFR-PERF-05) |
| Git processes | One per active clone/push; pooled `cat-file` per repo, closed after 60 s idle |
| DB connections | 1 writer + read pool (SQLite WAL) |
| Disk | Repos + logs + blobs; blob GC daily; artifact expiry hourly |
| Ports | 3000 (public, configurable), 3001 (loopback internal API) |

Install, configuration, backup and upgrade procedures: [reference/11-configuration-and-operations.md](reference/11-configuration-and-operations.md).

---

## 13. Concurrency Viewpoint

| Activity | Execution unit | Synchronisation |
|---|---|---|
| HTTP request | goroutine per request | `context` cancellation on client disconnect |
| Git transfer | child process per request | killed on context cancel |
| Runner long-poll | request goroutine waiting on notifier | broadcast wake-up; CAS claim decides |
| Background tasks | worker pool (default 4) | DB-leased tasks; lease expiry retries crashed work |
| DB writes | single writer connection | short transactions only |
| Per-repository maintenance vs push | per-repo lock | maintenance skipped while locked |
| SSE subscribers | goroutine per subscriber | slow subscribers dropped, never block the bus |

---

## 14. Security Viewpoint

| Concern | Design measure | SRS |
|---|---|---|
| Credential storage | argon2id passwords; SHA-256 tokens; AES-256-GCM secrets | NFR-SEC-01 |
| Session attacks | HttpOnly SameSite cookies, CSRF header, rotation at login | NFR-SEC-02 |
| Injection | argv-only process execution; `--end-of-options`; name validation | NFR-SEC-03 |
| XSS | React escaping, DOMPurify, CSP, attachment disposition for raw files | NFR-SEC-04 |
| Fork pull requests | No secrets, read-only token, approval gate | FR-CI-14 |
| Job isolation | Per-job workspace, short-lived job token, dedicated OS user per runner | NFR-SEC-06, ASM-3 |
| Internal API | Loopback only + per-process token | NFR-SEC-07 |

Threat model and full controls: [reference/10-security.md](reference/10-security.md).

---

## 15. Portability Viewpoint

OS-specific behaviour is isolated in four packages with `_windows.go` / `_unix.go` files:

| Package | Windows | Linux |
|---|---|---|
| `runner/procgroup` | Job Object, `TerminateJobObject` | process group, `SIGTERM`/`SIGKILL` |
| `runner/shell` | `pwsh` → Windows PowerShell 5.1, `.ps1` with BOM | `bash` → `sh` |
| `fsutil` | clear read-only, retry on sharing violation, owner-only ACL | `chmod 0600`, plain rename |
| `service` | Service Control Manager | systemd unit |

Complete difference catalogue: [reference/12-cross-platform.md](reference/12-cross-platform.md).

---

## 16. Design Rationale

| ADR | Decision | Main reason |
|---|---|---|
| ADR-001 | Go for server | Single static binary for Windows/Linux; Gitea and GitLab Runner as references |
| ADR-002 | TypeScript + React + Vite for UI | Ecosystem for diagrams, diffs, live logs |
| ADR-003 | Runner in Go, same repository | Shared IR types; process-tree control |
| ADR-004 | Use the `git` executable | Exact Git behaviour, protocol v2, performance |
| ADR-005 | SQLite (pure Go driver), PostgreSQL later | No DB server; no cgo |
| ADR-006 | One IR, two pipeline parsers | Both formats behave identically |
| ADR-007 | Server-Sent Events | One-way live data over plain HTTP |
| ADR-008 | Modular monolith | Easy deployment, clear boundaries |
| ADR-009 | `chi` router | Standard `net/http`, minimal |
| ADR-010 | TOML config + env overrides | Easy, unambiguous editing |

Full ADRs with alternatives considered: [reference/02-tech-stack-decisions.md](reference/02-tech-stack-decisions.md).

---

## 17. Module Design Specifications

Each module specification lists purpose, provided interface, required interfaces, data owned and error behaviour. **Function-level signatures and documentation comments for every planned exported function** are specified in [reference/14-code-structure.md §4](reference/14-code-structure.md); they become the code's doc comments when implemented.

| Module | Purpose | Provides | Requires | Owns | Errors returned |
|---|---|---|---|---|---|
| `config` | Load and validate configuration | `Load`, `Default`, `Validate` | — | — | list of invalid keys |
| `store` | Persistence | `Open`, `Migrate`, `Tx`, query methods | SQLite | all tables | `NotFound`, `Conflict` (unique) |
| `git/gitcmd` | Safe git execution | `Init`, `New`, `Output`, `Stream` | git binary | — | `ExitError` with stderr, timeout |
| `git` | Repository operations | `InitBare`, `Open`, tree/blob/log/diff, `MergeTree`, `CommitTree`, `UpdateRef` | `gitcmd` | repo directories | `NotFound`, `Conflict` (CAS) |
| `git/smarthttp` | Git transport | `ServeHTTP` | `auth`, `repos`, `gitcmd` | — | 401/403/404 |
| `git/hooks` | Push interception | `Install`, `RunClient`, `InternalAPI` | `repos` | hook scripts | non-zero exit with message |
| `auth` | Identity and permissions | `Login`, `ActorFromRequest`, tokens, `Permission`, `Require` | `store` | sessions, tokens | `Unauthenticated`, `Forbidden`, `NotFound` |
| `repos` | Repositories | lifecycle, content, protection, `CheckPush`, `AfterPush` | `git`, `store`, `events`, `auth` | repositories, collaborators, protection | `NotFound`, `Conflict`, `Invalid` |
| `issues` | Issue tracking | CRUD, comments, labels | `store`, `events`, `auth` | issues, comments, labels | `NotFound`, `Forbidden` |
| `pulls` | Pull requests | `Open`, `CheckMergeable`, `Merge`, `Review`, `OnPush` | `git`, `store`, `events`, `queue` | pull_requests | `Conflict` (merge/CAS) |
| `ci/parser/*` | Pipeline parsing | `Parse` | `ci/model`, `ci/expr` | — | parse errors with line numbers |
| `ci/trigger` | Start runs | `EvaluatePush`, `EvaluatePull`, `StartManual` | parsers, `git`, `scheduler` | — | invalid run records |
| `ci/scheduler` | Job orchestration | `CreateRun`, `Claim`, `AppendLog`, `FinishJob`, `CancelRun`, `Rerun` | `store`, `logs`, `events` | runs, jobs, steps, runners | `Conflict` (state), `Gap` (logs) |
| `secrets` | CI secrets | `Put`, `Names`, `ForJob` | `store`, cipher | secrets | `Forbidden` |
| `artifacts` / `releases` / `packages` / `deployments` | Publishing | upload, download, list | `blob`, `store`, `events` | their tables | `Conflict` (immutable) |
| `events` | Event bus | `Publish`, `Subscribe` | `store` | events | — |
| `queue` | Background work | `Enqueue`, `Handle`, `Start` | `store` | tasks | retries with backoff |
| `blob` | File storage | `Put`, `Open`, `GC` | filesystem | blobs/ | size limit exceeded |
| `explorer` | Tracing and visualisation | `StartSpan`, `Topology`, `Handler` | `events` | in-memory spans | — |
| `runner/*` | Job execution | see §4.2 | runner API | workspaces | job result codes |
