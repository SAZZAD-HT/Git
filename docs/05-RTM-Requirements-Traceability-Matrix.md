# Requirements Traceability Matrix (RTM)

**Project:** Anvil · **Document ID:** ANV-RTM-001 · **Version:** 1.0 · **Date:** 2026-10-30
**Traces:** [SRS](01-SRS-Software-Requirements-Specification.md) → [SDD](02-SDD-Software-Design-Description.md) → [STP](03-STP-Software-Test-Plan.md)

**Verification method:** T = Test, D = Demonstration, I = Inspection, A = Analysis. **Status:** all requirements are *Planned* until the phase that implements them closes; the status column is updated at each phase review.

## 1. Functional Requirements

| Req ID | Pri | Use case | Design (module · diagram) | Test cases | Ver. | Phase | Status |
|---|---|---|---|---|---|---|---|
| FR-AUTH-01 | M | UC-2 | auth · SD-01, CD-05 | TC-AUTH-01/02, TC-SMK-02 | T | P2 | Planned |
| FR-AUTH-02 | M | UC-2 | auth | TC-AUTH-04 | T | P2 | Planned |
| FR-AUTH-03 | S | UC-2 | users | TC-AUTH-05 | T | P2 | Planned |
| FR-AUTH-04 | M | UC-2 | auth (tokens) · CD-05 | TC-AUTH-06/07 | T | P2 | Planned |
| FR-AUTH-05 | M | UC-2 | httpserver middleware, auth | TC-AUTH-06/08 | T | P2 | Planned |
| FR-AUTH-06 | M | UC-4 | git/smarthttp, auth · SD-02 | TC-REPO-05/07 | T | P3 | Planned |
| FR-AUTH-07 | M | all | auth.Authorizer · CD-05 | TC-AUTH-07/09 | T | P2 | Planned |
| FR-AUTH-08 | M | UC-1 | auth.Authorizer | TC-AUTH-10 | T | P2 | Planned |
| FR-AUTH-09 | M | UC-2 | users, auth | TC-AUTH-11 | T | P2 | Planned |
| FR-AUTH-10 | S | UC-2 | auth | TC-AUTH-12 | T | P2 | Planned |
| FR-REPO-01 | M | UC-3 | repos, git · CD-02 | TC-REPO-01/02, TC-SMK-03 | T | P3 | Planned |
| FR-REPO-02 | M | UC-3 | repos | TC-REPO-03 | T | P3 | Planned |
| FR-REPO-03 | S | UC-3 | repos | TC-REPO-04 | T | P3 | Planned |
| FR-REPO-04 | M | UC-4 | git/smarthttp · SD-02 | TC-REPO-05/06, TC-SMK-03 | T | P3 | Planned |
| FR-REPO-05 | M | UC-4 | git/smarthttp, hooks · SD-03, AD-01 | TC-REPO-07, TC-SMK-03 | T | P3 | Planned |
| FR-REPO-06 | M | UC-4 | hooks, repos.CheckPush · AD-01 | TC-REPO-08/09/10 | T | P3 | Planned |
| FR-REPO-07 | M | UC-4 | repos.CheckPush · AD-01 | TC-REPO-11 | T | P3 | Planned |
| FR-REPO-08 | M | UC-1 | repos (content), git.Repository | TC-REPO-12 | T | P3 | Planned |
| FR-REPO-09 | M | UC-1 | web Markdown/CodeView | TC-REPO-13 | T | P3 | Planned |
| FR-REPO-10 | S | UC-3 | repos.PutFile, git.WriteFile | TC-REPO-14 | T | P3 | Planned |
| FR-REPO-11 | S | UC-1 | git.Archive | TC-REPO-15 | T | P3 | Planned |
| FR-REPO-12 | M | UC-3 | repos (collaborators) | TC-REPO-16 | T | P3 | Planned |
| FR-REPO-13 | C | UC-1 | git.Blame | — (Could) | D | P3 | Planned |
| FR-COLLAB-01 | M | UC-5 | issues | TC-COL-01 | T | P4 | Planned |
| FR-COLLAB-02 | M | UC-5 | store.NextIssueNumber | TC-COL-02/03 | T | P4 | Planned |
| FR-COLLAB-03 | M | UC-5 | issues | TC-COL-01 | T | P4 | Planned |
| FR-COLLAB-04 | S | UC-5 | issues | TC-COL-01 | T | P4 | Planned |
| FR-COLLAB-05 | M | UC-6 | pulls.Open · SD-04 | TC-COL-04 | T | P4 | Planned |
| FR-COLLAB-06 | M | UC-6 | pulls.Files/Commits | TC-COL-05 | T | P4 | Planned |
| FR-COLLAB-07 | M | UC-6 | pulls.CheckMergeable · SD-04, ST-02 | TC-COL-06 | T | P4 | Planned |
| FR-COLLAB-08 | S | UC-6 | pulls.Review | TC-COL-07 | T | P4 | Planned |
| FR-COLLAB-09 | M | UC-6 | pulls.Merge · SD-05, AD-05 | TC-COL-08/09 | T | P4 | Planned |
| FR-COLLAB-10 | S | UC-6 | pulls.Merge (checks) | TC-COL-10 | T | P5 | Planned |
| FR-COLLAB-11 | M | UC-6 | pulls.OnPush | TC-COL-11 | T | P4 | Planned |
| FR-CI-01 | M | UC-7 | ci/parser/github · CD-03 | TC-CI-01, TC-SMK-04 | T | P5 | Planned |
| FR-CI-02 | M | UC-7 | ci/parser/gitlab · CD-03 | TC-CI-02 | T | P5 | Planned |
| FR-CI-03 | M | UC-7 | ci/model (IR) | TC-CI-03 | T | P5 | Planned |
| FR-CI-04 | M | UC-7 | ci/trigger · AD-02 | TC-CI-04 | T | P5 | Planned |
| FR-CI-05 | M | UC-7 | ci/trigger · SD-06, AD-02 | TC-CI-05 | T | P5 | Planned |
| FR-CI-06 | M | UC-7 | ci/scheduler · AD-03, ST-01 | TC-CI-06 | T | P5 | Planned |
| FR-CI-07 | S | UC-7 | ci/trigger (matrix) | TC-CI-07 | T | P5 | Planned |
| FR-CI-08 | M | UC-12 | ci/scheduler.Claim · SD-07 | TC-CI-08/09 | T | P5 | Planned |
| FR-CI-09 | M | UC-7 | ci/logs, SSE · SD-07 | TC-CI-10, TC-SMK-04 | T | P5 | Planned |
| FR-CI-10 | M | UC-7 | ci/scheduler · SD-09 | TC-CI-12/13 | T | P5 | Planned |
| FR-CI-11 | S | UC-7 | ci/scheduler · AD-03 | TC-CI-14 | T | P5 | Planned |
| FR-CI-12 | M | UC-12 | scheduler loops · ST-01 | TC-CI-15/16 | T | P5 | Planned |
| FR-CI-13 | M | UC-7 | secrets · CD-05 | TC-CI-17 | T | P5 | Planned |
| FR-CI-14 | M | UC-7 | ci/trigger, secrets · AD-02 | TC-CI-19 | T | P5 | Planned |
| FR-CI-15 | M | UC-12 | runner masker · §11.6 | TC-CI-17/18 | T | P5 | Planned |
| FR-CI-16 | M | UC-11 | runnerapi.Register · ST-03 | TC-CI-20 | T | P5 | Planned |
| FR-CI-17 | M | UC-12 | runner shell · CD-04 | TC-CI-21/22 | T | P5 | Planned |
| FR-CI-18 | M | UC-12 | runner procgroup · SD-09 | TC-CI-12/23 | T | P5 | Planned |
| FR-CI-19 | M | UC-7 | api commit status, web | TC-CI-24 | T | P5 | Planned |
| FR-CI-20 | S | UC-7 | web PipelineGraph | TC-CI-25 | T | P5 | Planned |
| FR-CI-21 | M | UC-12 | runner actions | TC-CI-26 | T | P5 | Planned |
| FR-CI-22 | M | UC-7 | ci/parser/github | TC-CI-04 | T | P5 | Planned |
| FR-PUB-01 | M | UC-7 | artifacts, blob | TC-PUB-01/02 | T | P6 | Planned |
| FR-PUB-02 | M | UC-8 | releases · SD-08 | TC-PUB-03 | T | P6 | Planned |
| FR-PUB-03 | M | UC-8 | releases, runner actions, parser/gitlab | TC-PUB-04 | T | P6 | Planned |
| FR-PUB-04 | C | UC-8 | releases.GenerateNotes | TC-PUB-05 | T | P6 | Planned |
| FR-PUB-05 | S | UC-8 | packages | TC-PUB-06 | T | P6 | Planned |
| FR-PUB-06 | S | UC-9 | deployments · SD-08 | TC-PUB-07 | T | P6 | Planned |
| FR-PUB-07 | S | UC-8 | releases | TC-PUB-08 | T | P6 | Planned |
| FR-PUB-08 | S | UC-8 | releases, packages, blob | TC-PUB-03 | T | P6 | Planned |
| FR-ADM-01 | M | UC-11 | users | TC-ADM-01 | T | P2 | Planned |
| FR-ADM-02 | S | UC-11 | settings | TC-ADM-02 | T | P8 | Planned |
| FR-ADM-03 | S | UC-11 | events (audit) | TC-ADM-03 | T | P2 | Planned |
| FR-ADM-04 | C | UC-11 | queue, admin UI | — (Could) | D | P8 | Planned |
| FR-ADM-05 | M | UC-11 | service · DP-01/02 | TC-ADM-04, TC-SMK-01 | T | P1 | Planned |
| FR-ADM-06 | M | UC-11 | cmd backup/restore | TC-ADM-05 | T | P8 | Planned |
| FR-ADM-07 | S | UC-11 | cmd doctor | TC-ADM-06 | T | P1 | Planned |
| FR-ADM-08 | M | UC-11 | store.Migrate | TC-ADM-07 | T | P1 | Planned |
| FR-EXP-01 | S | UC-10 | explorer.Topology | TC-EXP-01 | T | P7 | Planned |
| FR-EXP-02 | S | UC-10 | explorer spans, SSE | TC-EXP-02 | T | P7 | Planned |
| FR-EXP-03 | C | UC-10 | explorer replay | TC-EXP-03 | T | P7 | Planned |
| FR-EXP-04 | C | UC-10 | web explorer tours | TC-EXP-04 | D | P7 | Planned |
| FR-EXP-05 | M | UC-10 | explorer + Authorizer | TC-EXP-05 | T | P7 | Planned |

## 2. Interface Requirements

| Req ID | Design | Test cases | Ver. |
|---|---|---|---|
| IR-1 | web client (SDD §4.3) | TC-NFR-08, system suites | T |
| IR-2 | api, OpenAPI (SDD §8) | route ⇔ OpenAPI test, all API tests | T |
| IR-3 | git/smarthttp | TC-REPO-05/06 | T |
| IR-4 | ci/runnerapi | TC-CI-08…16 | T |
| IR-5 | git/hooks internal API | TC-NFR-05, TC-REPO-08 | T |
| IR-6 | SSE streams | TC-CI-10, TC-EXP-02 | T |
| IR-7 | cmd/anvil, cmd/anvil-runner | TC-ADM-04…07 | T |
| IR-8 | service | TC-ADM-04 | T |
| IR-9 | mail (Could) | — | D |
| IR-10 | health, metrics | TC-SMK-01 | T |

## 3. Non-Functional Requirements

| Req ID | Design | Test cases | Ver. |
|---|---|---|---|
| NFR-PERF-01 | git cat-file pool, indexes (SDD §12) | TC-NFR-01 | A |
| NFR-PERF-02 | notifier + CAS claim (SDD §11.6) | TC-CI-28 | A |
| NFR-PERF-03 | SSE log streaming | TC-CI-10 | T |
| NFR-PERF-04 | streaming smart HTTP | TC-REPO-18/19 | A |
| NFR-PERF-05 | resource budget (SDD §12) | TC-NFR-02 | A |
| NFR-PERF-06 | startup sequence | TC-SMK-01 | T |
| NFR-SEC-01 | auth, secrets (SDD §14) | TC-AUTH-13 | I |
| NFR-SEC-02 | CSRF middleware | TC-AUTH-08 | T |
| NFR-SEC-03 | gitcmd argument rules | TC-REPO-02/17, TC-NFR-04 | T |
| NFR-SEC-04 | DOMPurify, CSP | TC-REPO-13 | T |
| NFR-SEC-05 | RateLimit middleware | TC-AUTH-02/03 | T |
| NFR-SEC-06 | checkout, job token lifecycle | TC-CI-27 | T |
| NFR-SEC-07 | internal listener on loopback | TC-NFR-05 | T |
| NFR-SEC-08 | dependency policy | TC-NFR-03 | A |
| NFR-SEC-09 | audit events | TC-ADM-03 | T |
| NFR-REL-01 | durable queue, DB-backed jobs | TC-ADM-08 | T |
| NFR-REL-02 | CAS claim | TC-CI-09 | T |
| NFR-REL-03 | update-ref CAS | TC-COL-09 | T |
| NFR-REL-04 | backup design | TC-ADM-05 | T |
| NFR-REL-05 | graceful shutdown | TC-ADM-09 | T |
| NFR-REL-06 | idempotent log append | TC-CI-11 | T |
| NFR-PORT-01 | portability viewpoint (SDD §15) | TC-NFR-06 | T |
| NFR-PORT-02 | pure-Go SQLite, CGO_ENABLED=0 | TC-NFR-07 | I |
| NFR-PORT-03 | portable names + paths | TC-ADM-05 | T |
| NFR-PORT-04 | CI OS matrix | TC-NFR-06 | T |
| NFR-USE-01 | design tokens, a11y rules | TC-NFR-08 | T |
| NFR-USE-02 | error contract (SDD §8) | TC-NFR-14 | I |
| NFR-USE-03 | install procedure | TC-ADM-10 | D |
| NFR-USE-04 | status icon + text | TC-NFR-08 | I |
| NFR-MAINT-01 | doc standards, lint | TC-NFR-09 | A |
| NFR-MAINT-02 | archtest | TC-NFR-09 | T |
| NFR-MAINT-03 | coverage gate | TC-NFR-10 | A |
| NFR-MAINT-04 | ADRs (SDD §16) | — | I |
| NFR-MAINT-05 | onboarding programme | — | D |
| NFR-OPS-01 | slog with request/trace IDs | TC-NFR-11 | T |
| NFR-OPS-02 | config loader | TC-NFR-12 | T |
| NFR-OPS-03 | runner protocol versioning | TC-NFR-13 | T |

## 4. Data Requirements

| Req ID | Design | Test cases | Ver. |
|---|---|---|---|
| DR-1 | store, git (SDD §7) | all integration suites | T |
| DR-2 | store conventions | unit tests of store | T |
| DR-3 | artifact expiry loop | TC-PUB-02 | T |
| DR-4 | events pruning | events unit test | T |
| DR-5 | blob store | blob unit tests | T |
| DR-6 | repos.Delete | TC-REPO-03 | T |

## 5. Coverage Summary

| Category | Requirements | With design element | With test / verification |
|---|---:|---:|---:|
| Functional | 77 | 77 | 75 test + 2 demonstration (Could items) |
| Interface | 10 | 10 | 10 |
| Non-functional | 37 | 37 | 37 |
| Data | 6 | 6 | 6 |

Every requirement has at least one design element and one verification activity. Orphan check: every test case in the STP maps to at least one requirement in this matrix.
