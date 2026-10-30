# Anvil

A self-hosted code forge written from scratch: GitHub-style repositories, issues and pull requests, plus GitLab-style CI/CD pipelines, releases and packages, in one program that runs natively on **Windows and Linux** with no Docker required.

> **Status: planning phase.** The design is complete and documented; implementation starts with Phase 1 of the [roadmap](docs/17-roadmap.md).

## Highlights (planned for v1.0)

- Git hosting over HTTP(S) using the standard `git` client
- Issues, pull requests, code review, merge and squash
- CI/CD that reads **both** `.github/workflows/*.yml` and `.gitlab-ci.yml`
- A cross-platform runner (PowerShell on Windows, bash on Linux) with live logs
- Artifacts, releases, a generic package registry, and deploy jobs
- A live **Architecture Explorer** that shows how every request moves through the system

## Tech stack

Go (server and runner) · TypeScript + React (frontend) · SQLite · the `git` CLI. Reasons are in [docs/02-tech-stack-decisions.md](docs/02-tech-stack-decisions.md).

## Documentation

Start at **[docs/README.md](docs/README.md)**. It lists every document in reading order, from vision and architecture to the function-level module reference and the onboarding guide for new developers.
