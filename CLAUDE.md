# CLAUDE.md

Guidance for Claude Code sessions (local and cloud) working in this repo.

## Project

**Anvil**: a from-scratch, self-hosted forge (GitHub-style collaboration + GitLab-style CI/CD) that runs natively on Windows and Linux without Docker. Go server and runner, React + TypeScript frontend, SQLite, the `git` CLI.

The design lives in `docs/`. Read `docs/README.md` first: the SRS (requirements), SDD (design, UML in `docs/diagrams/`) and RTM (traceability) are the baseline; `docs/reference/` holds detailed annexes. Docs are the source of truth until code exists. After that, code and docs must agree, and are updated in the same change.

## Rules

- Follow the layer and dependency rules in `docs/reference/14-code-structure.md` §2. Only `internal/git/gitcmd` runs git, only `internal/store` runs SQL.
- Every package has a `doc.go`. Every exported symbol has a doc comment meeting `docs/reference/15-documentation-standards.md` §2.
- Code must work on Windows and Linux. OS-specific code goes in `_windows.go` / `_unix.go` files and gets a row in `docs/reference/12-cross-platform.md`.
- Never build shell command strings from user input. Pass git arguments as lists, with user values after `--end-of-options` / `--` (`docs/reference/10-security.md` §5).
- Use `go run ./tools/devtool <task>` for build, test and lint (no Makefile or shell scripts).
- Branch off `main` for work; open a PR rather than pushing to `main` directly.
- Keep secrets out of the repo; use `.env` (git-ignored) and commit a `.env.example`.
