# 13 — Developer Onboarding

Welcome. This guide takes you from a fresh machine to your first merged change, and teaches you how Anvil works along the way. Plan on about one week.

## 1. Set up your machine

| Tool | Windows | Linux (Debian/Ubuntu) |
|---|---|---|
| Git ≥ 2.38 | [Git for Windows](https://git-scm.com/download/win) | `sudo apt install git` |
| Go (latest stable) | [go.dev/dl](https://go.dev/dl/) installer | [go.dev/dl](https://go.dev/dl/) tarball, add `/usr/local/go/bin` to `PATH` |
| Node.js 24 LTS | [nodejs.org](https://nodejs.org/) installer | NodeSource or `nvm` |
| Editor | VS Code with the Go, ESLint and Markdown Preview Mermaid extensions (any editor works) | same |

Then:

```bash
git clone <anvil repo url> && cd anvil
go run ./tools/devtool setup   # downloads Go modules and npm packages, installs golangci-lint
go run ./tools/devtool dev     # starts anvil on :3000 (auto-rebuild) + Vite on :5173 + one local runner
```

Open http://localhost:5173 and sign in as `admin` / `admin` (dev mode only; `dev` creates the user). The dev data directory is `./.devdata/`. Delete it to start fresh.

## 2. Training path

Each day has reading, a hands-on exercise, and a question you should be able to answer at the end. Ask your mentor if you can't.

### Day 1 — The big picture

- **Read:** [00 Vision](00-vision-and-scope.md), [01 Research](01-research.md), [03 Architecture](03-architecture-overview.md).
- **Do:** create a repo in the UI, clone it, push a commit, open the **Architecture Explorer** (`/explorer`) and push again while watching it.
- **Answer:** Which components does a `git push` pass through, in order? Why does the push return before CI starts?

### Day 2 — Git hosting

- **Read:** [04 Git hosting](04-git-hosting.md), then the code: `internal/git/smarthttp`, `internal/git/hooks`, `internal/git/gitcmd`.
- **Do:** run `GIT_TRACE_PACKET=1 git clone http://localhost:3000/admin/demo.git` (PowerShell: `$env:GIT_TRACE_PACKET=1`) and match the pkt-lines you see to §2 of the doc. Then protect `main` and try to force-push.
- **Answer:** How does the server know who pushed, inside the hook? What stops two merges from overwriting each other?

### Day 3 — Data and API

- **Read:** [05 Data model](05-data-model.md), [06 API](06-api-spec.md), [10 Security](10-security.md) §1–3.
- **Do:** create a PAT and use `curl` to create an issue. Open `.devdata/anvil.db` with any SQLite browser and find your issue and its number.
- **Answer:** Why does a private repo return 404 instead of 403? Why are tokens hashed with SHA-256 but passwords with argon2id?

### Day 4 — CI/CD

- **Read:** [07 CI/CD](07-ci-cd-design.md), [08 Publishing](08-publishing.md).
- **Do:** push `examples/demo-app` to your local Anvil. Watch the pipeline graph and live logs. Cancel a job halfway. Then switch from the GitHub-style file to the GitLab-style one and confirm the result is the same.
- **Answer:** Trace a job from `git.push` to `finish`: which functions in `ci/trigger`, `ci/scheduler` and `runner` run? How can two runners never get the same job?

### Day 5 — Frontend, cross-platform, and your first change

- **Read:** [09 Frontend](09-frontend-architecture.md), [12 Cross-platform](12-cross-platform.md), [15 Documentation standards](15-documentation-standards.md), [16 Testing](16-testing-strategy.md).
- **Do:** pick an issue labelled `good first issue`, write the failing test, fix it, update the docs, and open a PR.

## 3. How to trace any request through the code

1. **Start from the URL.** Search `internal/api/*.go` (REST), `internal/ci/runnerapi` (runner) or `internal/git/smarthttp` (Git) for the route.
2. The handler calls **one service method**. Jump to it in `internal/<service>`.
3. The service checks permission (`auth.Authorizer.Require`), then calls `store` (SQL), `git` (repository) or `blob` (files). It may `Publish` an event.
4. **For async effects**, search for subscribers of that event type (`events.Subscribe` / `queue.Handle`).
5. **Or skip the search:** open the Explorer, do the action, and click the trace. Each span names the package and function.

Every function on this path has a doc comment ([15](15-documentation-standards.md)). If one is missing or wrong, fixing it is a welcome first PR.

## 4. Everyday commands

| Task | Command |
|---|---|
| Run dev environment | `go run ./tools/devtool dev` |
| All tests | `go run ./tools/devtool test` |
| One Go package | `go test ./internal/ci/scheduler -run TestClaim -v` |
| Frontend tests | `cd web && npm test` |
| Lint everything | `go run ./tools/devtool lint` |
| Regenerate API types | `go run ./tools/devtool gen` |
| End-to-end tests | `go run ./tools/devtool e2e` |
| Build release binaries for all OSes | `go run ./tools/devtool release` |
| Browse generated docs | `go run ./tools/devtool docs` then open `build/docs/index.html` |

## 5. Working agreements

- Branch from `main`, open a PR, get one review. CI must pass on **both** Windows and Linux.
- Small PRs (under ~400 changed lines when possible).
- Docs and tests in the same PR as the code ([15 §6](15-documentation-standards.md)).
- Commit messages: imperative subject under 72 characters ("Add job retry to scheduler"), then a body explaining why.
- Found something confusing in these docs? Fix it. The next person will thank you.
