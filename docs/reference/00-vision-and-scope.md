# 00 — Vision and Scope

## 1. Problem

Teams who want to host their own code today choose between:

- **GitHub** — the best-known workflow, but cloud-only (GitHub Enterprise Server is commercial and Linux-appliance-only).
- **GitLab CE** — open source and powerful, but a large multi-service system that only runs on Linux and needs several gigabytes of RAM.
- **Gitea / Forgejo** — light and open source, but its CI runner executes jobs in Docker by default.

None of them is a single program you can run on a Windows laptop *and* a Linux server, with CI that works without Docker. Anvil fills that gap, and doubles as a **teaching codebase**: every part is documented so a new developer can learn how a code forge really works.

## 2. Vision

> One small program that gives a team GitHub-style collaboration and GitLab-style CI/CD on any Windows or Linux machine, understandable end to end by one developer.

## 3. Goals

| ID | Goal | Measured by |
|---|---|---|
| G1 | Host Git repositories over HTTP(S): clone, fetch, push with standard `git` clients | `git clone`/`git push` work against Anvil from Windows and Linux clients |
| G2 | GitHub-style collaboration: issues, pull requests, code review comments, merge | A PR can be opened, reviewed, and merged from the web UI |
| G3 | CI/CD that accepts **both** GitHub Actions-style (`.github/workflows/*.yml`) and GitLab-style (`.gitlab-ci.yml`) pipeline files | The same sample project passes using either file |
| G4 | Runs natively on Windows 10/11 and Linux (x86-64 and ARM64) with no Docker | Release binaries for both OSes; full test suite passes on both |
| G5 | Publish software: artifacts, releases, a generic package registry, and deploy jobs | A tag push produces a downloadable release |
| G6 | Make the system's internals visible: a live **Architecture Explorer** | A push is visibly traced through every component in the UI |
| G7 | Every package and function documented for training | Lint gate fails on undocumented code (see [15](15-documentation-standards.md)) |

## 4. Non-goals (version 1.0)

These are deliberately out of scope so v1.0 can ship. Each could be a later phase (see [17-roadmap.md](17-roadmap.md)).

- Being API-compatible with GitHub or GitLab. We copy *concepts and file formats*, not their APIs.
- Running third-party GitHub Actions from the marketplace (`uses: someone/action@v1`). Only a fixed set of built-in actions is emulated.
- Docker/container job execution (planned as an optional executor after 1.0).
- SSH Git transport (planned after 1.0; HTTP(S) only in 1.0).
- Organizations, teams, and fine-grained org permissions (users + collaborators only in 1.0).
- Federation, mirroring, Git LFS, wikis, project boards.
- Horizontal scaling across several servers. One server process, any number of runners.

## 5. Users and roles

| Role | Description |
|---|---|
| **Visitor** | Not signed in. Can browse public repositories if the instance allows it. |
| **User** | Has an account. Owns repositories, opens issues and PRs. |
| **Collaborator** | A user granted `read`, `write`, or `admin` on someone else's repository. |
| **Site admin** | Manages users, runners, and instance settings. |
| **Runner** | A machine running `anvil-runner` that executes CI jobs. Not a person, but a first-class actor with its own credentials. |

## 6. Glossary

| Term | Meaning in Anvil |
|---|---|
| **Forge** | A web application that hosts Git repositories plus collaboration tools (GitHub, GitLab, Gitea are forges). |
| **Bare repository** | A Git repository with no working tree (`git init --bare`). The server stores only bare repos. |
| **Ref** | A named pointer to a commit, for example `refs/heads/main` (branch) or `refs/tags/v1.0` (tag). |
| **Smart HTTP** | The Git protocol over HTTP that `git clone https://…` uses. See [04](04-git-hosting.md). |
| **Pack** | Git's compressed bundle of objects sent during clone/fetch/push. |
| **Server hook** | A program Git runs on the server during a push (`pre-receive`, `post-receive`). |
| **Pipeline file** | `.github/workflows/*.yml` or `.gitlab-ci.yml` in a repository. |
| **Pipeline / Run** | One execution of a pipeline file for one commit. GitHub calls it a *workflow run*; GitLab calls it a *pipeline*. Anvil uses **run** in code and **pipeline** in the UI. |
| **Job** | A unit of work inside a run, executed by exactly one runner. Has steps. |
| **Step** | One command (or built-in action) inside a job. |
| **Runner** | The `anvil-runner` program; polls the server for jobs and executes them. |
| **Job token** | A short-lived credential a job uses to clone the repo and upload results. |
| **Artifact** | Files a job uploads for later jobs or for download. Expire after a retention period. |
| **Release** | A named, permanent version tied to a Git tag, with downloadable assets. |
| **Package** | A versioned file in the generic package registry. |
| **IR** | *Intermediate representation*: Anvil's internal pipeline model that both file formats are parsed into. |
| **Event** | A record that something happened (`git.push`, `ci.job.finished`). Drives triggers, activity feeds, and the Architecture Explorer. |

## 7. Constraints

- **C1** Must run on Windows without WSL, Docker, or a Unix compatibility layer — except Git for Windows, which every Windows Git user already has.
- **C2** Default install needs no external database server (embedded SQLite).
- **C3** Server and runner are each a single executable file.
- **C4** All dependencies must be open source with permissive licenses (MIT, BSD, Apache-2.0, ISC).
- **C5** Minimum supported `git` version on the server: **2.38** (needed for `git merge-tree --write-tree`, see [04](04-git-hosting.md)).

## 8. Success criteria for version 1.0

1. A fresh Windows 11 machine and a fresh Ubuntu machine can each install Anvil in under 10 minutes by following [11](11-configuration-and-operations.md).
2. The **end-to-end demo** works on both: create repo → push app → pipeline runs tests → tag `v1.0.0` → release with built binaries appears → deploy job starts the app.
3. A new developer, using only these docs, can explain the path of a `git push` through the system and fix a small bug within their first week ([13](13-onboarding.md)).
