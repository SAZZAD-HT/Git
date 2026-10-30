# Software Test Plan (STP)

**Project:** Anvil — Self-Hosted Code Forge and CI/CD Platform
**Document ID:** ANV-STP-001 · **Version:** 1.0 (Draft for approval) · **Date:** 2026-10-30
**Standard:** IEEE 829-2008 (Software and System Test Documentation) — Master Test Plan and Test Case Specification

## Document Control

| Version | Date | Author | Change |
|---|---|---|---|
| 1.0 | 2026-10-30 | Project team | Initial master test plan and test case catalogue |

| Role | Name | Signature / Date |
|---|---|---|
| QA Lead | | |
| Technical Lead | | |
| Product Owner | SAZZAD-HT | |

---

## 1. Introduction

### 1.1 Purpose
This plan defines how Anvil 1.0 is verified against [ANV-SRS-001](01-SRS-Software-Requirements-Specification.md): scope, approach, environments, pass/fail criteria, schedule and the catalogue of test cases. Each test case traces to requirements in the [RTM](05-RTM-Requirements-Traceability-Matrix.md).

### 1.2 Test Items

| Item | Version | Description |
|---|---|---|
| `anvil` (server) | 1.0 | Windows and Linux builds |
| `anvil-runner` | 1.0 | Windows and Linux builds |
| Web client | 1.0 | Embedded SPA |
| Installation and operations procedures | 1.0 | Install guide, backup/restore |

### 1.3 Features to Be Tested
All requirements with priority **Must** and **Should** in the SRS (§4–§7).

### 1.4 Features Not to Be Tested
Items marked **Won't** or out of scope in SRS §1.2 (SSH, organisations, webhooks, LFS, Docker executor, marketplace actions). macOS builds are compiled but not tested.

---

## 2. Approach

### 2.1 Test Levels

| Level | Objective | Tools | Owner | When |
|---|---|---|---|---|
| Unit | Verify functions in isolation (parsers, expressions, authz, masking, pkt-line, state derivation) | `go test`, Vitest | Developers | Every commit |
| Integration | Verify a module with real SQLite, real `git`, real HTTP | `go test -tags integration`, `httptest` | Developers | Every commit |
| System (E2E) | Verify complete scenarios through the UI, git client and runner | Playwright, Go E2E harness | QA | Every pull request |
| Non-functional | Performance, security, portability, accessibility | Benchmarks, `govulncheck`, fuzzing, axe | QA | Nightly + release |
| Acceptance | Product owner validates use cases | Scripted demonstration | Product Owner | End of each phase |

### 2.2 Test Types

Functional · Regression · Security (static analysis, fuzzing, abuse cases) · Performance (load and soak) · Portability (Windows/Linux matrix) · Installation · Recovery (crash/restart, backup/restore) · Usability/accessibility.

### 2.3 Test Design Techniques
Equivalence partitioning and boundary values (names, sizes, limits); state transition testing (job, PR, runner state machines); decision tables (permission matrix, trigger matching); golden-file testing (pipeline parser output); race testing (concurrent claims, concurrent merges); fault injection (network drops, killed processes).

---

## 3. Pass/Fail Criteria

**Test case:** passes if all expected results are observed on **both Windows and Linux**.

**Release (exit) criteria for 1.0:**

1. 100% of Must test cases pass on both OSes.
2. ≥ 95% of Should test cases pass; failures have accepted workarounds.
3. No open defect of severity Critical or High.
4. NFR targets in SRS §6 met by measurement.
5. Coverage ≥ 80% lines for `ci`, `auth`, `git` packages.

**Suspension criteria:** testing pauses if the build fails to install on either OS, or if more than 20% of smoke tests fail. **Resumption:** after a fixed build passes the smoke suite (TC-SMK-*).

### Defect Severity

| Severity | Definition | Example |
|---|---|---|
| Critical | Data loss, security breach, or system unusable | Private repo readable anonymously |
| High | Major feature broken, no workaround | Pipelines never start |
| Medium | Feature impaired, workaround exists | Diff view wrong for renamed files |
| Low | Cosmetic or minor | Misaligned icon |

---

## 4. Test Environment

| Environment | OS | Purpose |
|---|---|---|
| ENV-W | Windows 11 Pro x64, Git for Windows ≥ 2.38, Windows PowerShell 5.1 (and separately with PowerShell 7) | Server + runner |
| ENV-L | Ubuntu 24.04 x64, Git ≥ 2.38, bash | Server + runner |
| ENV-MIX | Linux server + one Windows runner + one Linux runner + one deploy runner | Distributed CI/CD scenarios |
| ENV-PERF | Reference hardware: 4 cores, 8 GB RAM, SSD | Performance tests |
| Browsers | Chrome, Edge, Firefox (latest) | UI tests |
| Git minimum | Git 2.38.0 exactly | Compatibility with the minimum version |

Test data: generated repositories (small, 10k files, 100k commits, 1 GB), sample pipelines in both formats, the `examples/demo-app` project.

---

## 5. Test Case Specification

Format: **ID — Title — Preconditions — Steps — Expected result — Requirements — Level — Priority.** Steps are summarised; detailed scripts live with the automated tests.

### 5.1 Smoke (SMK)

| ID | Title | Steps | Expected result | Req | Level |
|---|---|---|---|---|---|
| TC-SMK-01 | Install and start | `anvil init`, `anvil serve` | `/readyz` returns 200 within 5 s | FR-ADM-05, NFR-PERF-06 | System |
| TC-SMK-02 | Admin login | Create admin via CLI; log in via UI | Dashboard shown | FR-AUTH-01 | System |
| TC-SMK-03 | Create, clone, push | Create repo; clone; commit; push | Push succeeds; file visible in UI | FR-REPO-01/04/05/08 | System |
| TC-SMK-04 | Pipeline runs | Push repo with one-job pipeline; runner online | Run succeeds; log visible | FR-CI-01, FR-CI-09 | System |

### 5.2 Authentication and Access (AUTH)

| ID | Title | Steps | Expected result | Req | Level |
|---|---|---|---|---|---|
| TC-AUTH-01 | Valid login | POST login with correct credentials | 200, session cookie HttpOnly + SameSite=Lax | FR-AUTH-01 | Integration |
| TC-AUTH-02 | Invalid login, no enumeration | Wrong password; unknown user | Same 401 message and similar timing | FR-AUTH-01, NFR-SEC-05 | Integration |
| TC-AUTH-03 | Login rate limit | 11 failed logins in 1 min from one IP | 429 with Retry-After | NFR-SEC-05 | Integration |
| TC-AUTH-04 | Logout | Logout; reuse cookie | 401 | FR-AUTH-02 | Integration |
| TC-AUTH-05 | Registration toggle | Register with setting off / on | 403 / 201 | FR-AUTH-03 | Integration |
| TC-AUTH-06 | PAT lifecycle | Create token; use; revoke; use again | Value shown once; works; then 401 | FR-AUTH-04/05 | Integration |
| TC-AUTH-07 | PAT scope enforcement | Push with `repo:read` token | Push refused | FR-AUTH-04, FR-AUTH-07 | Integration |
| TC-AUTH-08 | CSRF required | Cookie-auth POST without X-CSRF-Token | 403 | NFR-SEC-02 | Integration |
| TC-AUTH-09 | Permission matrix | For each actor × visibility × role, call read/write/admin endpoints | Matches decision table in SRS FR-AUTH-07 | FR-AUTH-07 | Unit |
| TC-AUTH-10 | Private repo hidden | Anonymous and non-collaborator GET private repo | 404 (not 403) | FR-AUTH-08 | Integration |
| TC-AUTH-11 | Password change revokes sessions | Two sessions; change password in one | Other session gets 401 | FR-AUTH-09 | Integration |
| TC-AUTH-12 | Session list and revoke | List sessions; revoke one | Revoked session gets 401 | FR-AUTH-10 | Integration |
| TC-AUTH-13 | Credential storage | Inspect DB after creating user, token, secret | argon2id hash, SHA-256 token hash, ciphertext only | NFR-SEC-01 | Inspection |

### 5.3 Repositories and Git (REPO)

| ID | Title | Steps | Expected result | Req | Level |
|---|---|---|---|---|---|
| TC-REPO-01 | Create repository | Create public and private repo with README | Bare repo on disk, README committed | FR-REPO-01 | Integration |
| TC-REPO-02 | Invalid names | `CON`, `..`, `a/b`, `x.git`, 101 chars | 422 for each | FR-REPO-01, NFR-SEC-03 | Unit |
| TC-REPO-03 | Rename / visibility / delete | Rename; make private; delete with wrong then right confirmation | Directory renamed; access changes; 422 then deleted | FR-REPO-02 | Integration |
| TC-REPO-04 | Fork | Fork public repo | New repo with same refs | FR-REPO-03 | Integration |
| TC-REPO-05 | Clone protocol v0 and v2 | Clone with `-c protocol.version=0` and `=2` | Both succeed | FR-REPO-04 | Integration |
| TC-REPO-06 | Shallow and partial clone | `--depth=1`, `--filter=blob:none` | Succeed | FR-REPO-04 | Integration |
| TC-REPO-07 | Push requires write | Push as reader | Rejected (403) | FR-REPO-05 | Integration |
| TC-REPO-08 | Protected branch force push | Force-push to protected `main` | "remote:" error; ref unchanged | FR-REPO-06 | Integration |
| TC-REPO-09 | Protected branch deletion | `git push origin :main` | Rejected | FR-REPO-06 | Integration |
| TC-REPO-10 | Require PR | Direct push to branch requiring PR | Rejected with explanation | FR-REPO-06 | Integration |
| TC-REPO-11 | refs/pull read-only | Push to `refs/pull/1/head` | Rejected | FR-REPO-07 | Integration |
| TC-REPO-12 | Browse content | Tree, file, history, commit diff, branches, tags | Correct content incl. renames, binary detection | FR-REPO-08 | System |
| TC-REPO-13 | Markdown sanitising | README with `<script>`, `javascript:` link, SVG onload | Rendered without executing script | FR-REPO-09, NFR-SEC-04 | System |
| TC-REPO-14 | Web edit with stale head | Edit file with outdated expected sha | 409, no commit | FR-REPO-10 | Integration |
| TC-REPO-15 | Zip download | Download archive of tag | Valid zip, matching files | FR-REPO-11 | Integration |
| TC-REPO-16 | Collaborators | Add read/write/admin; remove | Permissions change immediately | FR-REPO-12 | Integration |
| TC-REPO-17 | Argument injection | Ref `--upload-pack=x`, path `-rf` in API calls | Rejected / treated as literal; no command executed | NFR-SEC-03 | Integration |
| TC-REPO-18 | Large push streaming | Push 1 GB repo while measuring server RSS | Succeeds; memory rise < 100 MB | NFR-PERF-04 | Non-functional |
| TC-REPO-19 | Client disconnect | Kill clone mid-transfer | git process terminated on server | NFR-PERF-04 | Integration |

### 5.4 Collaboration (COLLAB)

| ID | Title | Steps | Expected result | Req | Level |
|---|---|---|---|---|---|
| TC-COL-01 | Issue lifecycle | Open, comment, label, close, reopen | States and events correct | FR-COLLAB-01/03/04 | System |
| TC-COL-02 | Shared numbering | Create issue, PR, issue | Numbers 1, 2, 3 | FR-COLLAB-02 | Integration |
| TC-COL-03 | Concurrent numbering | 50 parallel issue creations | 50 unique consecutive numbers | FR-COLLAB-02 | Integration |
| TC-COL-04 | Open PR same repo / fork | Open both kinds | `refs/pull/n/head` created | FR-COLLAB-05 | Integration |
| TC-COL-05 | PR diff and commits | Open PR with rename + binary | Files and commits correct | FR-COLLAB-06 | System |
| TC-COL-06 | Mergeability | Clean PR; conflicting PR | `clean`; `conflict` with file list | FR-COLLAB-07 | Integration |
| TC-COL-07 | Line review | Comment on line, approve | Comment anchored to line/side | FR-COLLAB-08 | System |
| TC-COL-08 | Merge strategies | Merge with merge-commit; squash another | Correct parents and messages | FR-COLLAB-09 | Integration |
| TC-COL-09 | Concurrent merge CAS | Two merges into `main` simultaneously | One succeeds, one 409; no lost commit | FR-COLLAB-09, NFR-REL-03 | Integration |
| TC-COL-10 | Required checks | Merge before checks pass | Refused with reason | FR-COLLAB-10 | Integration |
| TC-COL-11 | PR sync on push | Push to head branch | PR head and mergeability updated | FR-COLLAB-11 | Integration |

### 5.5 CI/CD (CI)

| ID | Title | Steps | Expected result | Req | Level |
|---|---|---|---|---|---|
| TC-CI-01 | GitHub parser golden files | Parse each fixture | IR equals golden JSON | FR-CI-01 | Unit |
| TC-CI-02 | GitLab parser golden files | Parse each fixture (anchors, extends, include local) | IR equals golden JSON | FR-CI-02 | Unit |
| TC-CI-03 | Format parity | Parse equivalent GitHub/GitLab pairs | Identical IR | FR-CI-03 | Unit |
| TC-CI-04 | Invalid file reporting | Cycle in needs, unknown need, bad YAML, unsupported `uses:` | Run "invalid" with file:line message | FR-CI-04, FR-CI-22 | Integration |
| TC-CI-05 | Triggers | Push to matching/non-matching branch, tag, paths, `[skip ci]`, manual | Runs created only when expected | FR-CI-05 | Integration |
| TC-CI-06 | DAG and conditions | Pipeline with needs, failure, `always`, `on_failure`, manual | States follow ST-01 | FR-CI-06 | Integration |
| TC-CI-07 | Matrix | 2×2 matrix with exclude | 3 jobs with correct values | FR-CI-07 | Unit |
| TC-CI-08 | Label matching | Jobs needing `windows`, `linux`, `deploy` with 3 runners | Each job on correct runner only | FR-CI-08 | System |
| TC-CI-09 | Claim race | 20 runners poll for 1 job, repeated 1,000 times | Always exactly one claim | FR-CI-08, NFR-REL-02 | Integration |
| TC-CI-10 | Live logs | Job printing a line per second; watch UI | Lines appear within 2 s | FR-CI-09, NFR-PERF-03 | System |
| TC-CI-11 | Log idempotency | Resend chunks; send gap | No duplicates; gap returns 416 + size | NFR-REL-06 | Integration |
| TC-CI-12 | Cancel run | Cancel during long step | Job "cancelled"; all child processes gone | FR-CI-10, FR-CI-18 | System |
| TC-CI-13 | Re-run failed only | Re-run after one failure | New run; only failed jobs execute | FR-CI-10 | Integration |
| TC-CI-14 | Automatic retry | Job failing once, `retry: 1` | Second attempt succeeds; both logs kept | FR-CI-11 | Integration |
| TC-CI-15 | Timeout | Job exceeding timeout | Failed "timed out"; processes killed | FR-CI-12 | System |
| TC-CI-16 | Runner lost | Kill runner process mid-job | Job failed "runner lost" after 120 s | FR-CI-12 | System |
| TC-CI-17 | Secrets | Set secret; echo it in job | Available to job; shown as `***` in log | FR-CI-13, FR-CI-15 | System |
| TC-CI-18 | Masking across chunks | Secret split across two log writes | Never appears unmasked | FR-CI-15 | Unit |
| TC-CI-19 | Fork PR restrictions | Fork PR printing secrets / pushing with token | Waits for approval; no secrets; push refused | FR-CI-14 | System |
| TC-CI-20 | Runner registration | Register with token; reuse beyond max uses; disable | Works; refused; disabled runner gets no jobs | FR-CI-16 | Integration |
| TC-CI-21 | Shells | Steps in pwsh, powershell, cmd, bash, sh | Each runs with correct semantics and exit codes | FR-CI-17 | System |
| TC-CI-22 | GitLab per-line failure on PowerShell | Script `[exit 3, echo after]` | Fails with 3; "after" not printed | FR-CI-17 | System |
| TC-CI-23 | Process tree kill | Step starts grandchild `sleep 600`; cancel | No surviving processes (OS process list) | FR-CI-18 | System |
| TC-CI-24 | Commit status | Run on commit and PR | Status icon on commit and PR | FR-CI-19 | System |
| TC-CI-25 | Run graph | Open run page | Jobs and edges match needs | FR-CI-20 | System |
| TC-CI-26 | Built-in actions | checkout (depth, ref), upload/download artifact | Behave as specified | FR-CI-21 | System |
| TC-CI-27 | Job token not on disk | After checkout, search workspace for token | Not found | NFR-SEC-06 | System |
| TC-CI-28 | Job start latency | Queue job with idle runner, 100 samples | p95 ≤ 2 s | NFR-PERF-02 | Non-functional |

### 5.6 Publishing (PUB)

| ID | Title | Steps | Expected result | Req | Level |
|---|---|---|---|---|---|
| TC-PUB-01 | Artifacts pass between jobs | Upload in build, download in test | Same bytes | FR-PUB-01 | System |
| TC-PUB-02 | Artifact expiry | Retention 1 day; advance clock | Row and blob removed | FR-PUB-01, DR-3 | Integration |
| TC-PUB-03 | Release via API/UI | Create release with assets | Visible, downloadable, checksums shown | FR-PUB-02, FR-PUB-08 | System |
| TC-PUB-04 | Release from pipeline (both formats) | Tag push with `anvil/release` and with GitLab `release:` | Same release produced | FR-PUB-03 | System |
| TC-PUB-05 | Generated notes | Two tags with commits between | Notes list commit subjects | FR-PUB-04 | Integration |
| TC-PUB-06 | Package immutability | Publish file; publish same file again | 201 then 409 | FR-PUB-05 | Integration |
| TC-PUB-07 | Deployment record | Job with environment succeeds / fails | Environment page shows correct status | FR-PUB-06 | System |
| TC-PUB-08 | Latest release URL | Publish v1 then v2; download latest asset | v2 asset | FR-PUB-07 | Integration |
| TC-PUB-09 | End-to-end release | Demo app: tag → build on Windows + Linux → release → manual deploy | All stages succeed on ENV-MIX | UC-8, UC-9 | System |

### 5.7 Administration and Operations (ADM)

| ID | Title | Steps | Expected result | Req | Level |
|---|---|---|---|---|---|
| TC-ADM-01 | User admin | Create, deactivate, promote | Deactivated user cannot log in | FR-ADM-01 | System |
| TC-ADM-02 | Settings | Change setting in UI | Takes effect without restart | FR-ADM-02 | Integration |
| TC-ADM-03 | Audit log | Perform security actions | Entries with actor + IP | FR-ADM-03, NFR-SEC-09 | Integration |
| TC-ADM-04 | Service install | `install-service` on both OSes; reboot | Service running after reboot | FR-ADM-05, IR-8 | System |
| TC-ADM-05 | Online backup/restore | Backup during active pushes; restore on other OS | Restored instance consistent; repos fsck clean | FR-ADM-06, NFR-REL-04, NFR-PORT-03 | System |
| TC-ADM-06 | Doctor | Break permissions / old git; run `doctor` | Problems reported with fixes | FR-ADM-07 | System |
| TC-ADM-07 | Migrations | Start new version on old DB | Migrated automatically | FR-ADM-08 | Integration |
| TC-ADM-08 | Crash recovery | Kill server during queued tasks and jobs | After restart nothing lost; jobs continue | NFR-REL-01 | System |
| TC-ADM-09 | Graceful shutdown | Stop during push | Push completes within 30 s | NFR-REL-05 | System |
| TC-ADM-10 | Install time | Fresh VM, follow guide | ≤ 10 minutes | NFR-USE-03 | Demonstration |

### 5.8 Architecture Explorer (EXP)

| ID | Title | Steps | Expected result | Req | Level |
|---|---|---|---|---|---|
| TC-EXP-01 | Topology | Open Explorer | All components with live stats | FR-EXP-01 | System |
| TC-EXP-02 | Live push trace | Push while watching | Spans: smarthttp → git → hook → internal API → event → trigger → scheduler → runner | FR-EXP-02 | System |
| TC-EXP-03 | Replay | Select past trace; step through | Spans in order with timings | FR-EXP-03 | System |
| TC-EXP-04 | Guided tour | Run "What happens on git push" | Nodes highlighted in order | FR-EXP-04 | System |
| TC-EXP-05 | Explorer authorisation | User without access to private repo watches stream | No events from that repo | FR-EXP-05 | Integration |

### 5.9 Non-Functional (NFR)

| ID | Title | Method | Expected result | Req |
|---|---|---|---|---|
| TC-NFR-01 | API latency | Load test on ENV-PERF | p95 ≤ 300 ms | NFR-PERF-01 |
| TC-NFR-02 | Capacity | 50 users + 50 runners for 1 h | No errors; RSS ≤ 1 GB | NFR-PERF-05 |
| TC-NFR-03 | Vulnerability scan | `govulncheck`, `npm audit` | No High/Critical | NFR-SEC-08 |
| TC-NFR-04 | Fuzzing | Fuzz parsers, expression engine, pkt-line for 1 h each | No crash or hang | NFR-SEC-03 |
| TC-NFR-05 | Internal API exposure | Connect to internal port from another host | Connection refused | NFR-SEC-07 |
| TC-NFR-06 | Cross-platform matrix | Full suite on ENV-W and ENV-L | All pass | NFR-PORT-01/04 |
| TC-NFR-07 | No cgo | Inspect build flags / `go version -m` | `CGO_ENABLED=0` for all targets | NFR-PORT-02 |
| TC-NFR-08 | Accessibility | axe on all pages + keyboard walkthrough | No WCAG 2.2 AA violations | NFR-USE-01, NFR-USE-04 |
| TC-NFR-09 | Doc coverage | Lint for doc comments, archtest | 0 violations | NFR-MAINT-01/02 |
| TC-NFR-10 | Code coverage | Coverage report | ≥ 80% for ci, auth, git | NFR-MAINT-03 |
| TC-NFR-11 | Structured logs | Inspect log lines for requests | request_id and trace_id present | NFR-OPS-01 |
| TC-NFR-12 | Env overrides | Set every key via env | Effective config matches | NFR-OPS-02 |
| TC-NFR-13 | Runner version skew | Server N with runner N-1 | Jobs run | NFR-OPS-03 |
| TC-NFR-14 | Error messages | Review all error paths in UI | Each states cause and action | NFR-USE-02 |

---

## 6. Test Deliverables

Master Test Plan (this document) · automated test code in the repository · test data fixtures · CI test reports per OS · performance report · security scan report · defect log · Test Summary Report per phase and for release 1.0.

## 7. Testing Tasks and Schedule

| Phase (see [SPMP](04-SPMP-Software-Project-Management-Plan.md)) | Test focus | Test cases |
|---|---|---|
| P1 Foundation | Smoke, install, migrations, archtest | TC-SMK-01, TC-ADM-04/07, TC-NFR-07/09/11/12 |
| P2 Identity | Auth suite | TC-AUTH-* |
| P3 Git hosting | Repo suite | TC-REPO-*, TC-SMK-03 |
| P4 Collaboration | Collab suite | TC-COL-* |
| P5 CI/CD | CI suite | TC-CI-*, TC-SMK-04 |
| P6 Publishing | Publishing suite | TC-PUB-* |
| P7 Explorer | Explorer suite | TC-EXP-* |
| P8 Hardening | Full regression + non-functional + acceptance | All, TC-NFR-*, TC-ADM-05/08/09/10 |

## 8. Roles and Responsibilities

| Role | Responsibility |
|---|---|
| Developers | Unit and integration tests for their changes; fix defects |
| QA Lead | Maintain this plan, system tests, environments, test summary reports |
| Product Owner | Acceptance testing at each phase review |
| Technical Lead | Review test design for architecture-critical areas (scheduler, git, security) |

## 9. Risks and Contingencies

| Risk | Contingency |
|---|---|
| Flaky tests on Windows (file locks, timing) | Quarantine label + mandatory fix within one week; retries forbidden for Must tests |
| Test environment drift | Environments defined as scripts in the repository; rebuilt weekly |
| Insufficient performance hardware | Use cloud VM matching reference spec for release measurement |

## 10. Approvals

See Document Control.
