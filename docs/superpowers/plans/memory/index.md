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
| MEM-0A | REQ-MEM-13 | Pinned upstream revisions (agent-diva, laputa, mem0, memU, TencentDB) + drift reconciliation | none | [MEM-0A](MEM-0A.md) | Done 2026-09-26 | `docs/research/2026-09-26-memory-upstream-pin.md` |
| MEM-0B | REQ-MEM-1,6,7,8 | Capability profile + trusted-scope/identity mapping contract | MEM-0A (pinned upstream facts) | [MEM-0B](MEM-0B.md) | Done 2026-09-26 | `docs/architecture/VIVY-MEMORY-PROFILE.md` |
| MEM-0C | REQ-MEM-9,10,11 | Host contracts closing GAP-A (persona projection) and GAP-B (session export) | MEM-0B (profile terms) | [MEM-0C](MEM-0C.md) | Done 2026-09-26 | `docs/architecture/VIVY-MEMORY-HOST-CONTRACTS.md` |
| MEM-0D | REQ-MEM-4,10 | Mutation authority resolution under Grants; Eino/EinoExt and Laputa readiness revalidation | MEM-0A | [MEM-0D](MEM-0D.md) | Done 2026-09-26 | `docs/research/2026-09-26-memory-g0-readiness.md` |
| MEM-0E | REQ-MEM-13 | Trackers and architecture docs updated; G0 outcomes folded back | MEM-0A–0D | [MEM-0E](MEM-0E.md) | Done 2026-09-26 | `docs/TODO.md`, `docs/DEFER.MD`, `docs/research/OPEN-ITEMS.md`, SCX cross-refs; log `docs/logs/2026-09-26-memory-g0-contracts/` |
| MEM-1 | REQ-MEM-1,6,7,13 | **Phase 1 (authorized):** BML standalone Go library port — record model, typed SQLite+FTS5 store, MemoryHome facade, migration adapters. No Port wiring. | none (library-only scope; G0 contracts govern later adapter Stories) | [MEM-1](MEM-1.md) | Done 2026-09-26 | `bml/` module + `bml/provider.go` trait contract + `bml/README.md`; PR #62; `go test` green; log `docs/logs/2026-09-26-memory-bml/notes.md` |
| MEM-1A | REQ-MEM-2–5,9 | Vivy adapter wiring for BML (context-source/observer/control-action/status + UI rewire) | MEM-1, MEM-0B, MEM-0C, MEM-0D | — | Ready | G0 contracts frozen + `bml/` landed — both preconditions met |
| MEM-2 | REQ-MEM-7,8,9 | One pinned remote provider (mem0) proving the profile | MEM-1A | — | Blocked | plan written after G1 evidence |
| MEM-3 | REQ-MEM-11 | Independent Laputa persona governance | MEM-0C, MEM-0D | — | Blocked | needs GAP-A contract + upstream readiness verdict |
| MEM-4 | REQ-MEM-12 | Garden connector + structured recall mapping | MEM-0C, MEM-3 | — | Blocked | needs governance path + Garden contract inspection |
| MEM-5 | REQ-MEM-4,10 | memU completed-session extraction; TencentDB mapping | MEM-0C, MEM-2 | — | Blocked | needs session export contract + provider proof |

Topological waves, as landed: phase 1 `{MEM-1}` ran first (user-directed:
restore BML as a standalone in-repo library before contract docs) and landed
2026-09-26 as `bml/`; the G0 doc Stories `{MEM-0A}`, `{MEM-0B, MEM-0D}`
(parallel), `{MEM-0C}`, `{MEM-0E}` all landed alongside. MEM-1A is the next
wave and is unblocked: its predecessors (MEM-1 + MEM-0B/0C/0D) are all in.

## Scope and readiness

Ready = contract docs written against the verified seams listed in the spec;
the gate for a doc Story is review plus `just ci` staying green (docs only).
No Go/TS code changes are authorized by G0 Stories. MEM-1 landed library-only
Go work per maintainer direction; the landed G0 contracts now gate the adapter
Story MEM-1A.

Iteration-log rule applies: each landed Story files `docs/logs/<date>-<slug>/`
(`summary.md` / `verification.md` / `acceptance.md`); unfinished gaps go to
`docs/TODO.md` §0.1.

## Decision log (append-only)

- 2026-09-26 — Package created from issue #33. Baseline `f6fb11bc`. Diva pin
  `0fd005a1`. `plugins/vivy-memory` confirmed UI-only (demo stubs); backend work
  assigned to a new `vivy/memory-bml` Module proposal rather than growing the
  shell.
- 2026-09-26 — Maintainer directed phase 1 = restore Diva BML, BML as a
  standalone library housed in this repo. MEM-1 rewritten from Blocked outline
  to executable plan (new `bml/` Go module, own `go.mod`, no Vivy Port wiring);
  adapter wiring split into future MEM-1A which does need the G0 contracts.
  Execution method: superpowers:subagent-driven-development.
