# Anvil — Project Documentation

**Anvil** is a self-hosted code forge, written from scratch, combining GitHub-style collaboration with GitLab-style CI/CD. It runs natively on Windows and Linux without Docker. This folder holds the complete engineering documentation set, written to IEEE standards.

## Document Register

| # | Document | ID | Standard | Version | Status |
|---|---|---|---|---|---|
| 1 | [Software Requirements Specification (SRS)](01-SRS-Software-Requirements-Specification.md) | ANV-SRS-001 | ISO/IEC/IEEE 29148:2018 | 1.0 | Draft for approval |
| 2 | [Software Design Description (SDD)](02-SDD-Software-Design-Description.md) | ANV-SDD-001 | IEEE 1016-2009 | 1.0 | Draft for approval |
| 3 | [Software Test Plan (STP)](03-STP-Software-Test-Plan.md) | ANV-STP-001 | IEEE 829-2008 | 1.0 | Draft for approval |
| 4 | [Software Project Management Plan (SPMP)](04-SPMP-Software-Project-Management-Plan.md) | ANV-SPMP-001 | ISO/IEC/IEEE 16326 / IEEE 1058 | 1.0 | Draft for approval |
| 5 | [Requirements Traceability Matrix (RTM)](05-RTM-Requirements-Traceability-Matrix.md) | ANV-RTM-001 | — | 1.0 | Draft for approval |
| 6 | [Diagram Catalogue (34 UML and other diagrams)](diagrams/README.md) | ANV-DIA-001 | UML 2.5, C4 | 1.0 | Draft for approval |
| 7 | [Technical Reference Annexes](reference/README.md) | ANV-REF-001 | — | 1.0 | Supporting detail |

## How the documents fit together

```
 SRS  ── what the system must do (requirements FR/NFR, use cases)
  │
  ├─► SDD  ── how it is designed (IEEE 1016 viewpoints + UML diagrams)
  │     └─► Reference annexes ── protocol, API, schema, function-level detail
  │
  ├─► STP  ── how each requirement is tested (test cases TC-*)
  │
  ├─► RTM  ── links every requirement → design element → test case
  │
  └─► SPMP ── who builds it, when, in which phases, and the risks
```

## Reading order

| Reader | Read |
|---|---|
| Product owner / approver | SRS §1–2, §5 → SPMP §1, §3 → RTM §5 |
| New developer (training) | SRS §1–2 → SDD (all, with diagrams) → [reference/13-onboarding.md](reference/13-onboarding.md) → [reference/14-code-structure.md](reference/14-code-structure.md) |
| Tester | SRS §4–6 → STP → RTM |
| Operator | SDD §12 → [reference/11-configuration-and-operations.md](reference/11-configuration-and-operations.md) |

## Viewing the diagrams

Every diagram is a PNG and SVG image in [`diagrams/png/`](diagrams/png/) and [`diagrams/svg/`](diagrams/svg/), embedded in the documents above. They display in any Markdown preview (VS Code: open a file and press **Ctrl+Shift+V**), on GitHub/GitLab, and in a browser. Editable sources are in [`diagrams/src/`](diagrams/src/).
