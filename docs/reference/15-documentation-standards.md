# 15 — Documentation Standards

Documentation is a deliverable, not an afterthought. A change is not done until its documentation is done, and CI enforces this where it can.

## 1. What must be documented

| Level | Where | Required content | Enforced by |
|---|---|---|---|
| **Package** | `doc.go` in every Go package | What the package owns, what it must not do, main types, link to the docs section | lint: `revive` `package-comments` |
| **Exported function / method / type** | Go doc comment | What it does, parameters with non-obvious meaning, return values, **errors it returns**, side effects (DB writes, events published, files touched), concurrency safety | lint: `revive` `exported` |
| **Unexported function** | Go doc comment | Required when longer than ~10 lines or not obvious from its name; always for functions with side effects | code review |
| **Non-obvious code inside a function** | Inline comment | **Why**, not what: the constraint, bug or protocol rule the code satisfies | code review |
| **TypeScript exports** (components, hooks, helpers) | TSDoc `/** … */` | Purpose, props/params, for hooks: query key and what invalidates it | `eslint-plugin-jsdoc` (`require-jsdoc` on exports) |
| **REST endpoint** | `api/openapi.yaml` | Summary, params, request/response schema, error codes, auth | test: routes ⇔ OpenAPI |
| **Config key** | [11 §3](11-configuration-and-operations.md) + struct tag comment | Meaning, default, unit | test: every `Config` field appears in the docs |
| **Event type** | `internal/events/catalog.go` + [03 §6](03-architecture-overview.md) | When emitted, payload fields | code review |
| **DB migration** | header comment in the `.sql` file | What changes and why | code review |
| **Architecture decision** | new ADR in [02](02-tech-stack-decisions.md) | Context, options, decision, consequences | code review |
| **OS-specific behaviour** | [12](12-cross-platform.md) | One row per difference | checklist in PR template |
| **User-visible change** | `CHANGELOG.md` under *Unreleased* | One line per change | PR template |

## 2. Go doc comment style

Follow the official [Go Doc Comments](https://go.dev/doc/comment) guide. A comment starts with the name it documents and is written in full sentences.

**Good:**

```go
// UpdateRef moves ref to newSHA only if it currently points to oldSHA.
// This compare-and-swap makes concurrent merges safe: the second one fails
// instead of silently overwriting the first (docs/04-git-hosting.md §5.3).
//
// An empty oldSHA means the ref must not exist yet. It returns errs.Conflict
// if the ref moved, and errs.Invalid if ref is not a valid ref name.
// UpdateRef does not run server hooks; callers must publish git.push themselves.
func (r *Repository) UpdateRef(ctx context.Context, ref, newSHA, oldSHA string) error
```

**Bad:**

```go
// UpdateRef updates a ref.        ← repeats the name, says nothing about CAS, errors or hooks
func (r *Repository) UpdateRef(ctx context.Context, ref, newSHA, oldSHA string) error
```

**Checklist for every exported function comment:**

1. First sentence: what it does, starting with the function name.
2. Why it exists or the rule it enforces, if not obvious. Link the docs section.
3. Meaning of parameters whose meaning isn't obvious from type and name (empty string, zero, nil).
4. Every error kind a caller may want to handle.
5. Side effects: writes to DB, disk, events published, processes started.
6. Concurrency: safe for concurrent use? Blocks? Respects `ctx` cancellation?

## 3. Package `doc.go` template

```go
// Package smarthttp serves the Git smart HTTP protocol (clone, fetch, push over HTTP).
//
// It authenticates the caller, checks repository permission, then streams the
// request body into `git upload-pack` or `git receive-pack` and streams the
// result back. It never parses pack data itself.
//
// Responsibilities:
//   - URL routing for /{owner}/{repo}.git/{info/refs,git-upload-pack,git-receive-pack}
//   - gzip request decoding and protocol v2 negotiation (Git-Protocol header)
//   - passing pusher identity to hooks through ANVIL_* environment variables
//
// Not responsible for: branch protection (internal/repos via hooks),
// reading repository content for the UI (internal/git).
//
// Design: docs/04-git-hosting.md §2–3.
package smarthttp
```

## 4. TypeScript / TSDoc style

```ts
/**
 * Live log of one CI job. Loads existing text, then follows the SSE stream.
 *
 * Query key: `['job', jobId, 'log']`. Appends to the cache as chunks arrive;
 * stops when the server sends `done`.
 *
 * @param jobId - Job to follow.
 * @returns The log text so far, the step ranges, and whether the job is still running.
 */
export function useJobLog(jobId: number): JobLogState { … }
```

## 5. Generated reference documentation

| Source | Tool | Output |
|---|---|---|
| Go doc comments | `pkgsite` (or `go doc` locally) | Browsable package reference |
| OpenAPI | Redoc (static HTML) | API reference page, also served by Anvil at `/api/docs` |
| TSDoc | TypeDoc | Frontend reference |
| `docs/*.md` | rendered by any Markdown viewer; Mermaid diagrams inline | Design docs |

`go run ./tools/devtool docs` builds all of it into `build/docs/` for local browsing.

## 6. Keeping docs true

- **Same pull request rule:** a change to behaviour updates the docs in the same PR. Reviewers reject PRs that don't.
- **PR template checklist:**
  - [ ] Doc comments added or updated for every changed exported symbol
  - [ ] `docs/` updated (which file: ___)
  - [ ] OpenAPI updated (if API changed)
  - [ ] Cross-platform table updated (if OS-specific)
  - [ ] CHANGELOG entry
- **Automated checks in CI:** Go lint (`revive` exported + package comments), ESLint jsdoc rule, OpenAPI ⇔ routes test, config ⇔ docs test, Markdown link checker (no broken links between docs), Mermaid syntax check.
- **Quarterly docs review:** walk through [13](13-onboarding.md) with the newest team member and fix whatever confused them.

## 7. Writing style

- Write for a developer who joined this week: explain *why*, define terms on first use or link the glossary ([00 §6](00-vision-and-scope.md)).
- Short sentences. Active voice. Present tense ("the scheduler claims a job", not "will claim").
- Prefer a table or a diagram over a long paragraph for anything with structure.
- Every diagram has a sentence before it saying what it shows.
- Code examples must work as written. Example scripts are tested in CI where practical.
