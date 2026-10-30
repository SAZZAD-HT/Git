# Software Requirements Specification (SRS)

**Project:** Anvil — Self-Hosted Code Forge and CI/CD Platform
**Document ID:** ANV-SRS-001 · **Version:** 1.0 (Draft for approval) · **Date:** 2026-10-30
**Standard:** ISO/IEC/IEEE 29148:2018 (Requirements engineering), SRS content outline

## Document Control

| Version | Date | Author | Change |
|---|---|---|---|
| 0.1 | 2026-10-30 | Project team | Initial requirements draft |
| 1.0 | 2026-10-30 | Project team | Restructured to IEEE 29148; use cases, NFRs and traceability added |

| Role | Name | Signature / Date |
|---|---|---|
| Product Owner | SAZZAD-HT | |
| Technical Lead | | |
| QA Lead | | |

---

## Table of Contents

1. [Introduction](#1-introduction)
2. [Overall Description](#2-overall-description)
3. [External Interface Requirements](#3-external-interface-requirements)
4. [Functional Requirements](#4-functional-requirements)
5. [Use Case Specifications](#5-use-case-specifications)
6. [Non-Functional Requirements](#6-non-functional-requirements)
7. [Data Requirements](#7-data-requirements)
8. [Verification](#8-verification)
9. [Appendices](#9-appendices)

---

## 1. Introduction

### 1.1 Purpose

This SRS specifies the functional and non-functional requirements of **Anvil**, version 1.0. It is the agreed baseline for design ([ANV-SDD-001](02-SDD-Software-Design-Description.md)), testing ([ANV-STP-001](03-STP-Software-Test-Plan.md)) and project planning ([ANV-SPMP-001](04-SPMP-Software-Project-Management-Plan.md)). Every requirement has a unique ID and is traced to design and tests in the [Requirements Traceability Matrix](05-RTM-Requirements-Traceability-Matrix.md).

**Intended readers:** the product owner (approval), developers (design and implementation), testers (test case derivation), and new team members (training).

### 1.2 Scope

Anvil is a single self-hosted program that provides:

- Git repository hosting over HTTP(S) for standard `git` clients;
- GitHub-style collaboration: issues, pull requests, code review, merge;
- GitLab-style CI/CD: pipelines from `.github/workflows/*.yml` **or** `.gitlab-ci.yml`, executed by a cross-platform runner;
- Publishing: artifacts, releases, a generic package registry, and deployments;
- An Architecture Explorer that visualises the system's internal processing live.

It runs natively on **Windows and Linux** without Docker or an external database server.

**Out of scope for 1.0:** SSH Git transport, organisations/teams, webhooks, Git LFS, wikis, Docker-based job execution, marketplace GitHub Actions, API compatibility with GitHub/GitLab, horizontal scaling.

### 1.3 Definitions, Acronyms and Abbreviations

| Term | Definition |
|---|---|
| Artifact | Files uploaded by a CI job, kept for a retention period |
| Bare repository | Git repository without a working tree, as stored on a server |
| CAS | Compare-and-swap: update only if the current value equals an expected value |
| CI/CD | Continuous Integration / Continuous Delivery |
| IR | Intermediate Representation: Anvil's internal pipeline model |
| Job | Unit of CI work executed by one runner |
| PAT | Personal Access Token |
| PR | Pull Request |
| Run / Pipeline | One execution of a pipeline file for one commit |
| Runner | The `anvil-runner` program that executes jobs |
| Smart HTTP | The Git transfer protocol over HTTP |
| SPA | Single-Page Application (the web frontend) |
| SSE | Server-Sent Events (HTTP streaming from server to browser) |

Further terms: [reference/00-vision-and-scope.md §6](reference/00-vision-and-scope.md).

### 1.4 References

| ID | Document |
|---|---|
| [R1] | ISO/IEC/IEEE 29148:2018 — Requirements engineering |
| [R2] | IEEE 1016-2009 — Software Design Descriptions |
| [R3] | IEEE 829-2008 — Software and System Test Documentation |
| [R4] | Git documentation: `gitprotocol-http`, `githooks`, `git-merge-tree` |
| [R5] | GitHub Actions workflow syntax (public documentation) |
| [R6] | GitLab CI/CD `.gitlab-ci.yml` keyword reference |
| [R7] | WCAG 2.2 — Web Content Accessibility Guidelines |
| [R8] | Anvil technical reference annexes: [reference/README.md](reference/README.md) |

### 1.5 Document Overview

Section 2 describes the product context. Sections 3–4 list interface and functional requirements. Section 5 specifies use cases. Section 6 lists measurable quality requirements. Section 7 covers data. Section 8 defines how each requirement is verified.

**Requirement notation.** `FR-<AREA>-<n>` = functional, `NFR-<CAT>-<n>` = non-functional, `IR-<n>` = interface, `DR-<n>` = data. Priority uses **MoSCoW**: **M** = Must (release-blocking), **S** = Should, **C** = Could, **W** = Won't (this release).

---

## 2. Overall Description

### 2.1 Product Perspective

Anvil is a new, self-contained product. It interacts with standard Git clients, web browsers, its own runner program, and (optionally) an SMTP server.

![CX-01 System Context](diagrams/png/CX-01-system-context.png)

*Figure 2-1 — System context ([source](diagrams/src/CX-01-system-context.mmd))*

### 2.2 Product Functions (summary)

| Area | Summary |
|---|---|
| Identity | Accounts, sessions, personal access tokens, permissions |
| Repositories | Create, fork, browse, clone, push, protect branches, edit files |
| Collaboration | Issues, labels, comments, pull requests, review, merge |
| CI/CD | Parse two pipeline formats, trigger, schedule, execute, stream logs, cancel, retry |
| Publishing | Artifacts, releases, generic packages, deployments |
| Administration | Users, runners, settings, audit log, backup, diagnostics |
| Insight | Architecture Explorer with live traces and guided tours |

### 2.3 User Classes and Characteristics

![UC-01 System Use Case Overview](diagrams/png/UC-01-system-overview.png)

*Figure 2-2 — Actors and top-level use cases ([source](diagrams/src/UC-01-system-overview.mmd))*

| User class | Description | Technical skill | Frequency |
|---|---|---|---|
| Visitor | Unauthenticated; browses public content | Any | Occasional |
| Developer | Pushes code, opens issues and PRs, runs pipelines | Git + basic CI | Daily |
| Repository Admin | Owns repos: collaborators, protection, secrets, releases, deploys | Advanced | Weekly |
| Site Admin | Manages the instance: users, runners, settings | Advanced + ops | Weekly |
| Operator | Installs, backs up, upgrades the server (often the Site Admin) | Systems administration | Monthly |
| Runner (system) | Executes CI jobs | — | Continuous |
| Git Client (system) | Standard `git` program | — | Continuous |

### 2.4 Operating Environment

| Item | Requirement |
|---|---|
| Server and runner OS | Windows 10/11, Windows Server 2019+, Linux (x86-64, ARM64) |
| Git | Version 2.38 or later on server and runners (Git for Windows on Windows) |
| Browser | Current versions of Chrome, Edge, Firefox, Safari |
| Hardware (minimum) | 1 CPU core, 256 MB RAM for the server; disk as needed for repositories |
| Network | HTTP(S); runners only need outbound access to the server |

### 2.5 Design and Implementation Constraints

| ID | Constraint |
|---|---|
| CON-1 | Must run natively on Windows without WSL, Docker, or Unix compatibility layers (Git for Windows is permitted) |
| CON-2 | No external database server required (embedded SQLite) |
| CON-3 | Server and runner are each delivered as one executable file |
| CON-4 | Only permissively licensed dependencies (MIT, BSD, Apache-2.0, ISC) |
| CON-5 | Server implemented in Go; frontend in TypeScript/React (decision record: [reference/02](reference/02-tech-stack-decisions.md)) |
| CON-6 | Git operations are performed by the `git` executable, not a reimplementation |

### 2.6 User Documentation

Installation guide, administrator guide, user help pages in the web UI, API reference (OpenAPI), and developer onboarding guide.

### 2.7 Assumptions and Dependencies

| ID | Assumption / dependency |
|---|---|
| ASM-1 | Users have Git installed and know basic commands |
| ASM-2 | Runner machines have the build tools their pipelines need (Go, Node, .NET, …) |
| ASM-3 | Each runner runs under a dedicated OS account (security relies on OS user isolation) |
| ASM-4 | A single server instance is sufficient for the target size (≤ 200 users, ≤ 50 runners) |

---

## 3. External Interface Requirements

| ID | Interface | Requirement | Priority |
|---|---|---|---|
| IR-1 | User interface | A responsive web UI served by the server, usable at 360 px width and above, with light and dark themes | M |
| IR-2 | REST API | JSON HTTP API under `/api/v1`, described by an OpenAPI 3.1 document; the web UI uses only this API | M |
| IR-3 | Git transport | Git smart HTTP protocol (v0 and v2) at `/{owner}/{repo}.git` | M |
| IR-4 | Runner protocol | HTTPS long-poll API under `/api/runner/v1`, initiated by runners only | M |
| IR-5 | Internal hook API | HTTP on the loopback interface only, authenticated by a per-process secret | M |
| IR-6 | Live updates | Server-Sent Events for job logs, run status and Explorer traces | M |
| IR-7 | Command line | `anvil` and `anvil-runner` subcommands for install, service, admin, backup, diagnostics | M |
| IR-8 | Operating system services | Install as a Windows service or a systemd unit | M |
| IR-9 | Email | Optional SMTP notifications | C |
| IR-10 | Monitoring | `/healthz`, `/readyz`; Prometheus `/metrics` | S |

Detailed interface specifications: [reference/06-api-spec.md](reference/06-api-spec.md), [reference/04-git-hosting.md](reference/04-git-hosting.md), [reference/07-ci-cd-design.md §7](reference/07-ci-cd-design.md).

---

## 4. Functional Requirements

### 4.1 Identity and Access (AUTH)

| ID | Requirement | Priority |
|---|---|---|
| FR-AUTH-01 | The system shall allow a user to sign in with username and password and create a browser session. | M |
| FR-AUTH-02 | The system shall allow a user to sign out, ending the session. | M |
| FR-AUTH-03 | The system shall allow self-registration only when enabled by a site admin. | S |
| FR-AUTH-04 | The system shall allow a user to create, list and revoke personal access tokens with scopes and optional expiry; a token's value is shown only once. | M |
| FR-AUTH-05 | The system shall authenticate API calls with a session cookie (plus CSRF token) or a bearer token. | M |
| FR-AUTH-06 | The system shall authenticate Git HTTP requests with HTTP Basic credentials containing a PAT or job token. | M |
| FR-AUTH-07 | The system shall compute repository permission (none/read/write/admin) from ownership, collaborator role, visibility and site-admin status. | M |
| FR-AUTH-08 | The system shall return "not found" for private resources the caller cannot read. | M |
| FR-AUTH-09 | The system shall let a user change their password, revoking their other sessions. | M |
| FR-AUTH-10 | The system shall let a user list and revoke their active sessions. | S |

### 4.2 Repositories (REPO)

| ID | Requirement | Priority |
|---|---|---|
| FR-REPO-01 | The system shall let a user create a public or private repository, optionally with an initial README. | M |
| FR-REPO-02 | The system shall let a repository admin rename, change visibility, change the default branch, and delete a repository (with typed confirmation). | M |
| FR-REPO-03 | The system shall let a user fork a repository they can read. | S |
| FR-REPO-04 | The system shall serve `git clone` and `git fetch` to users with read access. | M |
| FR-REPO-05 | The system shall accept `git push` from users with write access. | M |
| FR-REPO-06 | The system shall enforce protected-branch rules on push: no force-push, no deletion, optional "require pull request". | M |
| FR-REPO-07 | The system shall reject client pushes to `refs/pull/*`. | M |
| FR-REPO-08 | The system shall show files, directories, commit history, single commits with diffs, branches and tags in the web UI. | M |
| FR-REPO-09 | The system shall render README files as sanitised Markdown and source files with syntax highlighting. | M |
| FR-REPO-10 | The system shall let users with write access create and delete branches, and edit a file in the browser with a commit message. | S |
| FR-REPO-11 | The system shall provide zip download of any revision. | S |
| FR-REPO-12 | The system shall let a repository admin add, change and remove collaborators with read/write/admin roles. | M |
| FR-REPO-13 | The system shall show blame for a file. | C |

### 4.3 Collaboration (COLLAB)

| ID | Requirement | Priority |
|---|---|---|
| FR-COLLAB-01 | The system shall let readers open issues with title and Markdown body. | M |
| FR-COLLAB-02 | The system shall number issues and pull requests from one shared per-repository sequence. | M |
| FR-COLLAB-03 | The system shall let users comment on issues and pull requests, and edit/delete their own comments. | M |
| FR-COLLAB-04 | The system shall support labels and filtering issues by state, label, author and text. | S |
| FR-COLLAB-05 | The system shall let a user open a pull request from a branch in the same repository or a fork. | M |
| FR-COLLAB-06 | The system shall show a pull request's commits and per-file diff. | M |
| FR-COLLAB-07 | The system shall determine and display whether a pull request merges cleanly, listing conflicting files otherwise. | M |
| FR-COLLAB-08 | The system shall let reviewers comment on specific diff lines and submit approve / request-changes reviews. | S |
| FR-COLLAB-09 | The system shall merge a pull request with "merge commit" or "squash" strategy, atomically, failing safely if the base branch moved. | M |
| FR-COLLAB-10 | The system shall block merging when required pipeline checks have not succeeded. | S |
| FR-COLLAB-11 | The system shall keep a pull request's head up to date when its source branch receives pushes. | M |

### 4.4 CI/CD (CI)

| ID | Requirement | Priority |
|---|---|---|
| FR-CI-01 | The system shall read pipeline definitions from `.github/workflows/*.yml` using the supported GitHub subset. | M |
| FR-CI-02 | The system shall read pipeline definitions from `.gitlab-ci.yml` using the supported GitLab subset. | M |
| FR-CI-03 | The system shall convert both formats into one internal model so that equivalent files produce identical runs. | M |
| FR-CI-04 | The system shall report invalid pipeline files on the commit with the file name, line and reason. | M |
| FR-CI-05 | The system shall start pipelines on push, pull request, tag and manual events, honouring branch, tag and path filters and `[skip ci]`. | M |
| FR-CI-06 | The system shall order jobs by `needs` and GitLab stages, and support conditions (on_success, on_failure, always, manual). | M |
| FR-CI-07 | The system shall expand matrix definitions into separate jobs. | S |
| FR-CI-08 | The system shall assign each job to exactly one runner whose labels include all required labels. | M |
| FR-CI-09 | The system shall stream job logs live to the browser and store them. | M |
| FR-CI-10 | The system shall let users with write access cancel a run, re-run all jobs, or re-run failed jobs. | M |
| FR-CI-11 | The system shall retry failed jobs automatically up to the configured retry count. | S |
| FR-CI-12 | The system shall enforce job timeouts and fail jobs whose runner stops responding. | M |
| FR-CI-13 | The system shall let repository admins store encrypted secrets and expose them only to jobs of that repository. | M |
| FR-CI-14 | The system shall withhold secrets from pull-request pipelines from forks and require approval before they run. | M |
| FR-CI-15 | The system shall mask secret values in job logs. | M |
| FR-CI-16 | The system shall let site admins and repository admins register runners with one-time registration tokens, and disable or delete runners. | M |
| FR-CI-17 | The runner shall execute steps with PowerShell (Windows) or bash/sh (Linux), or a shell named in the step. | M |
| FR-CI-18 | The runner shall terminate the entire process tree of a cancelled or timed-out job. | M |
| FR-CI-19 | The system shall show pipeline status on commits and pull requests. | M |
| FR-CI-20 | The system shall show each run as a job dependency graph. | S |
| FR-CI-21 | The runner shall provide built-in `actions/checkout`, `actions/upload-artifact`, `actions/download-artifact`, `anvil/release`, `anvil/publish-package`. | M |
| FR-CI-22 | The system shall reject unsupported `uses:` actions at parse time with a clear message. | M |

### 4.5 Publishing (PUB)

| ID | Requirement | Priority |
|---|---|---|
| FR-PUB-01 | The system shall store job artifacts, let later jobs download them, and delete them after their retention period. | M |
| FR-PUB-02 | The system shall let users and jobs create releases tied to Git tags, with notes and downloadable assets. | M |
| FR-PUB-03 | The system shall support the GitLab `release:` keyword and the `anvil/release` action. | M |
| FR-PUB-04 | The system shall generate release notes from commit subjects since the previous tag on request. | C |
| FR-PUB-05 | The system shall provide a generic package registry where published file versions are immutable. | S |
| FR-PUB-06 | The system shall record deployments per environment when jobs declare an environment, and show what is deployed where. | S |
| FR-PUB-07 | The system shall provide a stable URL for assets of the latest release. | S |
| FR-PUB-08 | The system shall show and verify SHA-256 checksums of release assets and packages. | S |

### 4.6 Administration (ADM)

| ID | Requirement | Priority |
|---|---|---|
| FR-ADM-01 | Site admins shall create, deactivate and promote users. | M |
| FR-ADM-02 | Site admins shall edit instance settings in the UI. | S |
| FR-ADM-03 | Site admins shall view an audit log of security-relevant actions. | S |
| FR-ADM-04 | Site admins shall inspect the background task queue. | C |
| FR-ADM-05 | Operators shall install and remove the server and runner as native OS services via the CLI. | M |
| FR-ADM-06 | Operators shall create and restore consistent backups while the server runs. | M |
| FR-ADM-07 | Operators shall run a diagnostics command that checks configuration, Git version, permissions and database integrity. | S |
| FR-ADM-08 | The server shall apply database migrations automatically at startup. | M |

### 4.7 Architecture Explorer (EXP)

| ID | Requirement | Priority |
|---|---|---|
| FR-EXP-01 | The system shall display a live diagram of its components with current statistics. | S |
| FR-EXP-02 | The system shall animate live request traces across components in processing order. | S |
| FR-EXP-03 | The system shall let users replay a recorded trace step by step. | C |
| FR-EXP-04 | The system shall provide guided tours explaining key flows (push, CI job). | C |
| FR-EXP-05 | Explorer data shall only include repositories the viewer can read. | M |

---

## 5. Use Case Specifications

### 5.1 Use Case Diagrams

![UC-02 Code Hosting and Collaboration](diagrams/png/UC-02-code-collaboration.png)

*Figure 5-1 — Code hosting and collaboration ([source](diagrams/src/UC-02-code-collaboration.mmd))*

![UC-03 CI/CD and Publishing](diagrams/png/UC-03-cicd-publishing.png)

*Figure 5-2 — CI/CD and publishing ([source](diagrams/src/UC-03-cicd-publishing.mmd))*

![UC-04 Administration and Insight](diagrams/png/UC-04-administration.png)

*Figure 5-3 — Administration and insight ([source](diagrams/src/UC-04-administration.mmd))*

### 5.2 Use Case Catalogue

| ID | Name | Primary actor | Requirements |
|---|---|---|---|
| UC-1 | Browse public repositories | Visitor | FR-REPO-08/09, FR-AUTH-08 |
| UC-2 | Sign in / manage account | Developer | FR-AUTH-01…10 |
| UC-3 | Manage repositories | Repository Admin | FR-REPO-01/02/03/12, FR-REPO-06 |
| UC-4 | Clone / fetch / push code | Git Client | FR-REPO-04…07, FR-AUTH-06 |
| UC-5 | Track issues | Developer | FR-COLLAB-01…04 |
| UC-6 | Review and merge pull requests | Developer | FR-COLLAB-05…11 |
| UC-7 | Run CI/CD pipelines | Developer | FR-CI-01…15, 19, 20 |
| UC-8 | Publish releases and packages | Repository Admin | FR-PUB-02…05, 07, 08 |
| UC-9 | Deploy to environments | Repository Admin | FR-PUB-06, FR-CI-06 (manual) |
| UC-10 | Explore system architecture | Visitor / Developer | FR-EXP-01…05 |
| UC-11 | Administer instance | Site Admin | FR-ADM-01…08, FR-CI-16 |
| UC-12 | Execute CI jobs | Runner | FR-CI-08, 09, 12, 17, 18, 21 |

### 5.3 Detailed Use Cases

#### UC-4 Push code

| Field | Description |
|---|---|
| Primary actor | Developer (through Git Client) |
| Preconditions | Repository exists; developer has write permission and a valid PAT |
| Trigger | Developer runs `git push` |
| Main success scenario | 1. Git client sends objects and ref updates over HTTPS. 2. System authenticates the developer and checks write permission. 3. System stores the objects. 4. System checks each ref update against protection rules. 5. System updates the refs. 6. System records the push, starts matching pipelines and updates affected pull requests. 7. Git client reports success, showing any server messages (e.g. "Create a pull request: URL"). |
| Alternative flows | 2a. Invalid credentials → 401, client prompts for credentials. 2b. No write permission → 403 (or 404 for private repos). 4a. Force-push or deletion of a protected branch, or a push to a branch requiring PRs → that ref is rejected with "remote: <reason>". 4b. Push to `refs/pull/*` → rejected. |
| Postconditions | Refs updated; `git.push` event recorded; pipelines evaluated asynchronously |
| Requirements | FR-REPO-05/06/07, FR-AUTH-06, FR-CI-05, FR-COLLAB-11 |
| Design | [SD-03](diagrams/png/SD-03-push.png), [AD-01](diagrams/png/AD-01-push-processing.png) |

#### UC-6 Review and merge a pull request

| Field | Description |
|---|---|
| Primary actor | Developer (author or reviewer); merge requires write permission |
| Preconditions | Source branch has commits not on the target branch |
| Main success scenario | 1. Author opens a PR choosing base and head. 2. System assigns the next number, records the head commit and checks mergeability in the background. 3. Pipelines for the PR start. 4. Reviewer views commits and diffs, adds line comments and approves. 5. Required checks succeed. 6. A user with write access chooses a strategy and merges. 7. System creates the merge (or squash) commit, moves the base branch atomically, closes the PR and starts base-branch pipelines. |
| Alternative flows | 2a. Conflicts → PR shows "cannot merge" and the file list. 5a. Checks failing → merge button disabled with reason. 6a. Head or base changed since the page loaded → 409, user reloads and retries. 6b. User lacks write access → merge not offered. |
| Postconditions | Base branch contains the changes; PR state "merged"; system comment recorded |
| Requirements | FR-COLLAB-05…11, FR-CI-19 |
| Design | [SD-04](diagrams/png/SD-04-open-pull-request.png), [SD-05](diagrams/png/SD-05-merge-pull-request.png), [AD-05](diagrams/png/AD-05-merge-pull-request.png), [ST-02](diagrams/png/ST-02-pull-request-states.png) |

#### UC-7 Run a CI/CD pipeline

| Field | Description |
|---|---|
| Primary actor | Developer |
| Preconditions | Repository contains a valid pipeline file; at least one runner with matching labels is online |
| Trigger | Push, PR, tag, or manual start |
| Main success scenario | 1. System reads pipeline files at the commit and parses them. 2. System matches triggers and filters. 3. System creates a run with jobs and queues jobs without dependencies. 4. Runners claim jobs (UC-12). 5. Developer watches the job graph and live logs. 6. As jobs finish, dependent jobs are queued or skipped. 7. Run finishes; status shown on commit and PR. |
| Alternative flows | 1a. File invalid → run created with status "invalid" and the error line. 2a. No trigger matches → no run. 3a. Fork PR → run waits for approval and gets no secrets. 4a. No matching runner → jobs stay queued and the UI says no runner matches the labels. 5a. Developer cancels → running jobs are terminated (SD-09). 6a. Job fails with retries left → re-queued. |
| Postconditions | Run, jobs, logs and artifacts stored |
| Requirements | FR-CI-01…15, 19, 20 |
| Design | [SD-06](diagrams/png/SD-06-pipeline-trigger.png), [AD-02](diagrams/png/AD-02-trigger-evaluation.png), [AD-03](diagrams/png/AD-03-scheduler-advance.png), [ST-01](diagrams/png/ST-01-job-states.png) |

#### UC-8 Publish a release

| Field | Description |
|---|---|
| Primary actor | Repository Admin (or a pipeline job on their behalf) |
| Preconditions | Write permission; tag exists or a target commit is given |
| Main success scenario | 1. Admin pushes tag `vX.Y.Z`. 2. Pipeline builds on Windows and Linux runners and uploads artifacts. 3. Release job creates the release and uploads assets. 4. System records checksums and publishes `release.published`. 5. Users download assets from the release page or the latest-release URL. |
| Alternative flows | 3a. Release for the tag already exists and is not a draft → asset upload with an existing name is rejected. 3b. Job from a fork → no write token, release creation refused. |
| Postconditions | Release visible with assets and checksums |
| Requirements | FR-PUB-02/03/04/07/08 |
| Design | [SD-08](diagrams/png/SD-08-release-and-deploy.png) |

#### UC-9 Deploy to an environment

| Field | Description |
|---|---|
| Primary actor | Repository Admin |
| Preconditions | A runner labelled for the target environment is installed on the target server; release exists |
| Main success scenario | 1. Deploy job waits in "manual" state. 2. Admin clicks Run. 3. Runner on the target downloads the release asset, replaces the service and health-checks it. 4. System records the deployment as successful. |
| Alternative flows | 3a. Health check fails → deploy script rolls back, job fails, deployment recorded as failed. 2a. Ref not protected but job targets a protected label → job refused. |
| Postconditions | Environment page shows the deployed version |
| Requirements | FR-PUB-06, FR-CI-06 |

#### UC-12 Execute a CI job

| Field | Description |
|---|---|
| Primary actor | Runner |
| Preconditions | Runner registered and online |
| Main success scenario | 1. Runner long-polls for work. 2. System atomically assigns a matching job and issues a job token. 3. Runner creates a workspace and checks out the exact commit. 4. For each step, runner renders a script for the selected shell, runs it, and streams masked output. 5. Runner uploads artifacts and reports the result. 6. System advances the run. 7. Runner deletes the workspace. |
| Alternative flows | 2a. Another runner claimed the job first → this runner tries the next candidate. 4a. Cancel signal received → runner kills the process tree and reports "cancelled". 4b. Network error → runner retries log upload from the last acknowledged offset. 4c. Runner crashes → server fails the job after 120 s ("runner lost"). |
| Postconditions | Job has a final status, a complete log and its artifacts |
| Requirements | FR-CI-08/09/12/15/17/18/21 |
| Design | [SD-07](diagrams/png/SD-07-job-execution.png), [AD-04](diagrams/png/AD-04-runner-job.png), [ST-03](diagrams/png/ST-03-runner-states.png) |

#### UC-1, UC-2, UC-3, UC-5, UC-10, UC-11 (summary form)

| UC | Main flow | Key alternatives |
|---|---|---|
| UC-1 Browse | Visitor opens a public repo → sees README, files, commits | Instance requires sign-in → redirected to login; private repo → 404 |
| UC-2 Account | Sign in (SD-01) → manage profile, password, tokens, sessions | Wrong credentials → generic error, rate limit after repeated failures |
| UC-3 Manage repo | Create → configure collaborators, protection, secrets → rename/delete | Name taken → 409; delete requires typing `owner/name` |
| UC-5 Issues | Open issue → label → comment → close/reopen | Non-author without write access cannot edit others' issues |
| UC-10 Explore | Open Explorer → see topology → perform action → watch trace → replay | Explorer disabled by admin → page explains how to enable |
| UC-11 Administer | Manage users, runners, settings → review audit log → back up | Last site admin cannot be demoted |

---

## 6. Non-Functional Requirements

Reference hardware for performance targets: 4 CPU cores, 8 GB RAM, SSD, on Windows 11 and Ubuntu 24.04.

### 6.1 Performance (PERF)

| ID | Requirement | Priority |
|---|---|---|
| NFR-PERF-01 | 95% of REST API reads complete in ≤ 300 ms for repositories ≤ 10,000 files and ≤ 100,000 commits. | M |
| NFR-PERF-02 | A queued job is assigned to an idle matching runner within 2 s (95th percentile). | M |
| NFR-PERF-03 | Live log output appears in the browser within 2 s of being produced. | M |
| NFR-PERF-04 | Clone and push throughput is at least 90% of a direct `git` transfer over the same link; packs are streamed, never fully buffered in memory. | M |
| NFR-PERF-05 | The server supports 50 concurrent active users and 50 connected runners with server memory ≤ 1 GB. | S |
| NFR-PERF-06 | Server cold start (with migrations already applied) completes in ≤ 5 s. | S |

### 6.2 Security (SEC)

| ID | Requirement | Priority |
|---|---|---|
| NFR-SEC-01 | Passwords are stored with argon2id; tokens with SHA-256; secrets with AES-256-GCM. No credential is stored in plain text. | M |
| NFR-SEC-02 | All state-changing cookie-authenticated requests require a valid CSRF token. | M |
| NFR-SEC-03 | User input is never passed to a shell; Git arguments are passed as lists with user values after `--end-of-options` / `--`. | M |
| NFR-SEC-04 | Rendered Markdown and raw file downloads cannot execute script in the application origin. | M |
| NFR-SEC-05 | Login and Git authentication are rate-limited per IP and per account. | M |
| NFR-SEC-06 | Job tokens are valid only for one repository and only while the job runs; they are never written to disk in the workspace. | M |
| NFR-SEC-07 | The internal hook API is reachable only from the loopback interface. | M |
| NFR-SEC-08 | Release builds contain no dependency with a known vulnerability rated High or Critical (`govulncheck`, `npm audit`). | M |
| NFR-SEC-09 | Security-relevant actions are recorded in the audit log with actor and IP. | S |

### 6.3 Reliability and Availability (REL)

| ID | Requirement | Priority |
|---|---|---|
| NFR-REL-01 | No accepted push, queued task or queued job is lost when the server restarts or crashes. | M |
| NFR-REL-02 | A job is never executed by two runners at the same time. | M |
| NFR-REL-03 | Concurrent merges into the same branch never overwrite each other. | M |
| NFR-REL-04 | Backups taken while the server runs are internally consistent and restore successfully. | M |
| NFR-REL-05 | Graceful shutdown completes in-flight requests and pushes within 30 s. | S |
| NFR-REL-06 | Log uploads are idempotent: resending a chunk never duplicates log content. | M |

### 6.4 Portability (PORT)

| ID | Requirement | Priority |
|---|---|---|
| NFR-PORT-01 | Server and runner run natively on Windows 10/11, Windows Server 2019+, and Linux x86-64/ARM64. | M |
| NFR-PORT-02 | Release binaries are built without cgo for all supported targets. | M |
| NFR-PORT-03 | A data directory backed up on one supported OS restores on the other. | S |
| NFR-PORT-04 | The full automated test suite passes on Windows and Linux for every change. | M |

### 6.5 Usability and Accessibility (USE)

| ID | Requirement | Priority |
|---|---|---|
| NFR-USE-01 | The web UI conforms to WCAG 2.2 level AA. | S |
| NFR-USE-02 | Every error message states what happened and what the user can do. | M |
| NFR-USE-03 | A new operator installs the server following the guide in ≤ 10 minutes on a clean machine. | M |
| NFR-USE-04 | Job status is conveyed by icon and text, never colour alone. | M |

### 6.6 Maintainability (MAINT)

| ID | Requirement | Priority |
|---|---|---|
| NFR-MAINT-01 | Every package and every exported function, type and TypeScript export has a documentation comment (enforced by lint). | M |
| NFR-MAINT-02 | Layer dependency rules are enforced by an automated architecture test. | M |
| NFR-MAINT-03 | Line coverage ≥ 80% for the `ci`, `auth` and `git` packages. | S |
| NFR-MAINT-04 | Every design decision is recorded as an ADR. | M |
| NFR-MAINT-05 | A new developer completes the onboarding programme and merges a first change within 5 working days. | S |

### 6.7 Supportability and Operations (OPS)

| ID | Requirement | Priority |
|---|---|---|
| NFR-OPS-01 | All logs are structured and carry a request ID and trace ID. | M |
| NFR-OPS-02 | Every configuration key can be overridden by an environment variable. | M |
| NFR-OPS-03 | Runners up to one minor version older than the server remain supported. | S |

---

## 7. Data Requirements

| ID | Requirement | Priority |
|---|---|---|
| DR-1 | The system shall persist metadata in an embedded SQLite database and Git objects in bare repositories on disk. | M |
| DR-2 | Timestamps shall be stored in UTC with millisecond precision. | M |
| DR-3 | Artifacts expire after a configurable retention period (default 30 days); releases and packages are kept until deleted. | M |
| DR-4 | Events are retained for a configurable period (default 90 days). | S |
| DR-5 | Uploaded files are stored content-addressed by SHA-256 and de-duplicated. | S |
| DR-6 | Deleting a repository removes its Git data, logs, artifacts, releases and packages. | M |

Logical data model: [ER-01](diagrams/png/ER-01-database.png); class view: [CD-01](diagrams/png/CD-01-domain-model.png); full schema: [reference/05-data-model.md](reference/05-data-model.md).

---

## 8. Verification

| Method | Code | Applied to |
|---|---|---|
| Test | T | Automated unit, integration and end-to-end tests ([ANV-STP-001](03-STP-Software-Test-Plan.md)) |
| Demonstration | D | Manual walkthrough at phase review (install time, onboarding, UI usability) |
| Inspection | I | Code / document review (documentation coverage, ADRs) |
| Analysis | A | Measurement and static analysis (performance, vulnerability scans, coverage) |

Every requirement's verification method and test case IDs are listed in the [RTM](05-RTM-Requirements-Traceability-Matrix.md).

---

## 9. Appendices

### Appendix A — Supported pipeline syntax

The exact supported subset of GitHub Actions and GitLab CI keywords is specified in [reference/07-ci-cd-design.md §1](reference/07-ci-cd-design.md).

### Appendix B — Diagram index

All diagrams are listed in [diagrams/README.md](diagrams/README.md).

### Appendix C — Open issues

| ID | Issue | Owner | Due |
|---|---|---|---|
| OI-1 | Confirm the product name ("Anvil" is a working name) | Product Owner | Before P1 |
| OI-2 | Confirm target team size (assumption ASM-4) | Product Owner | Before P1 |
| OI-3 | Decide whether fork-PR approval is required by default for public repositories | Product Owner | Before P5 |
