# Anvil

A self-hosted code forge written from scratch: GitHub-style repositories, issues and pull requests, plus GitLab-style CI/CD pipelines, releases and packages, in one program that runs natively on **Windows and Linux** with no Docker required.

> **Status: Phase 1 (Foundation) in progress.** The server starts, migrates its database and serves health and version endpoints. Plan: [SPMP](docs/04-SPMP-Software-Project-Management-Plan.md).

## Quick start

Needs Go 1.27+ and Git 2.38+. Node.js is optional (only for the web UI).

```bash
go run ./tools/devtool build                 # -> bin/anvil (bin\anvil.exe on Windows)
bin/anvil init --config .dev/anvil.toml      # config + data directory + secret key
bin/anvil serve --config .dev/anvil.toml     # http://localhost:3000
```

Check it: `curl localhost:3000/readyz` and `bin/anvil doctor --config .dev/anvil.toml`.

Other tasks: `go run ./tools/devtool test | lint | dist | dev | docs`.

## Highlights (planned for v1.0)

- Git hosting over HTTP(S) using the standard `git` client
- Issues, pull requests, code review, merge and squash
- CI/CD that reads **both** `.github/workflows/*.yml` and `.gitlab-ci.yml`
- A cross-platform runner (PowerShell on Windows, bash on Linux) with live logs
- Artifacts, releases, a generic package registry, and deploy jobs
- A live **Architecture Explorer** that shows how every request moves through the system

## Tech stack

Go (server and runner) · TypeScript + React (frontend) · SQLite · the `git` CLI. Reasons are in [docs/reference/02-tech-stack-decisions.md](docs/reference/02-tech-stack-decisions.md).

## Documentation

Start at **[docs/README.md](docs/README.md)**: the IEEE document set (SRS, SDD, STP, SPMP, RTM), UML diagrams and technical annexes. The same docs as a website: open `site/index.html`.
