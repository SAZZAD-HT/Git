# Technical Reference Annexes (ANV-REF-001)

These annexes support the formal documents in the parent folder ([SRS](../01-SRS-Software-Requirements-Specification.md), [SDD](../02-SDD-Software-Design-Description.md), [STP](../03-STP-Software-Test-Plan.md), [SPMP](../04-SPMP-Software-Project-Management-Plan.md), [RTM](../05-RTM-Requirements-Traceability-Matrix.md)). They hold the detailed specifications the formal documents refer to: protocols, API endpoints, database columns, configuration keys, cross-platform rules, and the function-level module reference.

If an annex and a formal document disagree, the formal document wins until the annex is corrected. The UML diagrams for everything here are in [../diagrams/](../diagrams/README.md); the inline Mermaid blocks in these annexes are working sketches.

## Who this is for

- **New developers** joining the project: follow the reading order below; it is also the training path in [13-onboarding.md](13-onboarding.md).
- **Reviewers** checking a change against the design.
- **Operators** installing Anvil on a Windows or Linux machine (start at [11-configuration-and-operations.md](11-configuration-and-operations.md)).

## Reading order

| # | Document | What you learn |
|---|---|---|
| 00 | [Vision and scope](00-vision-and-scope.md) | What Anvil is, what it is not, glossary |
| 01 | [Research: how GitHub, GitLab and Gitea work](01-research.md) | The systems we studied and what we borrow from each |
| 02 | [Technology decisions](02-tech-stack-decisions.md) | Which language for which part, and why (ADRs) |
| 03 | [Architecture overview](03-architecture-overview.md) | System context, containers, components, request flows |
| 04 | [Git hosting internals](04-git-hosting.md) | Smart HTTP protocol, server hooks, how a push is processed |
| 05 | [Data model](05-data-model.md) | Database schema, ER diagram, on-disk layout |
| 06 | [HTTP API specification](06-api-spec.md) | Every REST endpoint, auth, errors, pagination |
| 07 | [CI/CD engine design](07-ci-cd-design.md) | Pipeline formats, scheduler, runner protocol, job lifecycle |
| 08 | [Publishing: artifacts, releases, packages, deploy](08-publishing.md) | How a pipeline ships software |
| 09 | [Frontend architecture](09-frontend-architecture.md) | React app structure, state, routing, the Architecture Explorer |
| 10 | [Security model](10-security.md) | Authentication, authorization, secrets, threat model |
| 11 | [Configuration and operations](11-configuration-and-operations.md) | Config reference, install, backup, upgrades, logs |
| 12 | [Cross-platform design](12-cross-platform.md) | Every Windows vs Linux difference and how we handle it |
| 13 | [Developer onboarding](13-onboarding.md) | Setup, training path, how to trace a request through the code |
| 14 | [Code structure and module reference](14-code-structure.md) | Every package and every planned function, with responsibilities |
| 15 | [Documentation standards](15-documentation-standards.md) | How every function, package and change must be documented |
| 16 | [Testing strategy](16-testing-strategy.md) | Unit, integration, end-to-end, cross-platform test matrix |
| 17 | [Roadmap](17-roadmap.md) | Phases, milestones, deliverables and exit criteria |

## Diagrams

All diagrams are written in [Mermaid](https://mermaid.js.org/) inside the Markdown files, so they are versioned with the text and reviewed like code. They render on GitHub, GitLab, Gitea, and in VS Code with a Mermaid preview extension. Never commit a diagram only as an image: an image cannot be diffed or reviewed.

## Conventions used in these docs

- **MUST / SHOULD / MAY** follow [RFC 2119](https://www.rfc-editor.org/rfc/rfc2119) meaning.
- `code` font marks identifiers that will exist in the code: package names, functions, config keys, endpoints.
- "Planned" means the design is decided but not implemented yet. When a feature ships, remove the word.
- Paths like `internal/git/smarthttp` are relative to the repository root.
