# MEMORY implementation package

Epic: pluggable memory providers, independent Laputa governance, optional
Garden — issue #33, stage G0 first.
Code baseline: `f6fb11bc` (branch `feat/memory`, worktree `agent-vivy-memory`).
Spec: [2026-09-26 memory providers design](../../specs/2026-09-26-memory-providers-design.md).
Authorization: maintainer directed MEMORY track kickoff 2026-09-26. Planning
package production is in scope; publishing, push, merge, and issue comments are
not implied by it.

## Read order and authority

1. Repository `AGENTS.md`, then the spec above.
2. This index owns Story state and dependency data — no parallel diagram.
3. Each Story plan owns its implementation steps.
4. Symbols, file paths, and contract names in plans are proposals until the
   implementing Story lands them. No hidden chat context is required.

## Story graph and current state

| Story | Requirements | Outcome | Immediate predecessor | Plan | State | Evidence / next gate |
| --- | --- | --- | --- | --- | --- | --- |
| MEM-0A | REQ-MEM-13 | Pinned upstream revisions (agent-diva, laputa, mem0, memU, TencentDB) + drift reconciliation | none | [MEM-0A](MEM-0A.md) | Ready | research note under `docs/research/` |
| MEM-0B | REQ-MEM-1,6,7,8 | Capability profile + trusted-scope/identity mapping contract | MEM-0A (pinned upstream facts) | [MEM-0B](MEM-0B.md) | Ready | `docs/architecture/VIVY-MEMORY-PROFILE.md` |
| MEM-0C | REQ-MEM-9,10,11 | Host contracts closing GAP-A (persona projection) and GAP-B (session export) | MEM-0B (profile terms) | [MEM-0C](MEM-0C.md) | Ready | `docs/architecture/VIVY-MEMORY-HOST-CONTRACTS.md` |
| MEM-0D | REQ-MEM-4,10 | Mutation authority resolution under Grants; Eino/EinoExt and Laputa readiness revalidation | MEM-0A | [MEM-0D](MEM-0D.md) | Ready | `docs/research/2026-09-26-memory-g0-readiness.md` |
| MEM-0E | REQ-MEM-13 | Trackers and architecture docs updated; G0 outcomes folded back | MEM-0A–0D | [MEM-0E](MEM-0E.md) | Ready | `docs/TODO.md`, `docs/DEFER.MD`, SCX cross-refs |
| MEM-1 | REQ-MEM-1–9,13 | BML vertical slice: bounded recall + CRUD + scoped isolation + status | MEM-0B, MEM-0C, MEM-0D | [MEM-1](MEM-1.md) | Blocked — needs profile + mutation-authority + projection decisions | exact blockers recorded in plan |
| MEM-2 | REQ-MEM-7,8,9 | One pinned remote provider (mem0) proving the profile | MEM-1 | — | Blocked | plan written after G1 evidence |
| MEM-3 | REQ-MEM-11 | Independent Laputa persona governance | MEM-0C, MEM-0D | — | Blocked | needs GAP-A contract + upstream readiness verdict |
| MEM-4 | REQ-MEM-12 | Garden connector + structured recall mapping | MEM-0C, MEM-3 | — | Blocked | needs governance path + Garden contract inspection |
| MEM-5 | REQ-MEM-4,10 | memU completed-session extraction; TencentDB mapping | MEM-0C, MEM-2 | — | Blocked | needs session export contract + provider proof |

Topological waves: `{MEM-0A}`, `{MEM-0B, MEM-0D}` (parallel), `{MEM-0C}`,
`{MEM-0E}`, `{MEM-1}`, then `{MEM-2}` and `{MEM-3}` independently, `{MEM-4}`,
`{MEM-5}`. File conflicts: none among G0 Stories (disjoint new docs); MEM-0E
serializes after all contract docs so tracker text cites real paths.

## Scope and readiness

Ready = contract docs written against the verified seams listed in the spec;
the gate for a doc Story is review plus `just ci` staying green (docs only).
No Go/TS code changes are authorized by G0 Stories. MEM-1 stays Blocked until
MEM-0B/0C/0D produce their contracts — that is the point of the stage gate.

Iteration-log rule applies: each landed Story files `docs/logs/<date>-<slug>/`
(`summary.md` / `verification.md` / `acceptance.md`); unfinished gaps go to
`docs/TODO.md` §0.1.

## Decision log (append-only)

- 2026-09-26 — Package created from issue #33. Baseline `f6fb11bc`. Diva pin
  `0fd005a1`. `plugins/vivy-memory` confirmed UI-only (demo stubs); backend work
  assigned to a new `vivy/memory-bml` Module proposal rather than growing the
  shell.
