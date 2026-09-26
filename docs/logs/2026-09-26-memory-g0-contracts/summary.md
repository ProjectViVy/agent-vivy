# Memory G0 contract freeze + tracker foldback

Iteration: 2026-09-26. Scope: the whole G0 doc-story set of the MEMORY track
(issue #33) — MEM-0A through MEM-0E — plus the foldback that records MEM-1's
landing on the repo trackers. Docs only; no code changed in this iteration.

## What landed

| Story | Outcome | Evidence |
| --- | --- | --- |
| MEM-0A | Upstream revisions pinned (agent-diva `0fd005a1`, laputa `1e402835`, mem0 `94c3fe9f`, memU `2c050bc9`, TencentDB) + drift reconciliation | `docs/research/2026-09-26-memory-upstream-pin.md` |
| MEM-0B | Memory capability profile: standardized capability bundle over existing Ports, trusted-scope/identity mapping | `docs/architecture/VIVY-MEMORY-PROFILE.md` |
| MEM-0C | Host contracts closing GAP-A (persona projection) and GAP-B (completed-session export) | `docs/architecture/VIVY-MEMORY-HOST-CONTRACTS.md` |
| MEM-0D | Mutation authority under Grants; Eino/EinoExt + Laputa readiness revalidation at pinned revisions | `docs/research/2026-09-26-memory-g0-readiness.md` |
| MEM-0E | Tracker foldback: TODO/DEFER/OPEN-ITEMS stage rows, SCX cross-refs, this log | this directory + `docs/superpowers/plans/memory/index.md` |

MEM-1 landed earlier in the same phase: the standalone `bml/` Go module
(record model, typed SQLite+FTS5 store, maintenance, `MemoryHome` facade,
migration adapters, `bml/provider.go` trait contract, `bml/README.md`) — PR #62
on `feat/memory`. Its own iteration log already exists at
`docs/logs/2026-09-26-memory-bml/notes.md` and is not duplicated here.

## Tracker changes (MEM-0E)

- `docs/TODO.md` §0.1: added stage rows MEM-0A..MEM-5 — G0 stories DONE with
  doc paths, MEM-1 DONE with the `bml/` + PR #62 pointer, MEM-1A READY-NEXT
  (both preconditions now met), MEM-2..5 BLOCKED. UI-EVO row re-pointed at the
  landed memory package and profile.
- `docs/DEFER.MD`: MEM-1 umbrella row rewritten — BML library landed; what
  remains deferred is runtime wiring (MEM-1A) + later stages. AutoDream /
  Evolution / RAG moved to their own deferred ID `MEM-CAP`.
- `docs/research/OPEN-ITEMS.md`: MEM-1 row updated to match; `MEM-CAP` added.
- `docs/architecture/SCX-PLUGIN-INTEGRATION.md`: one-line frozen-contract
  pointers to `VIVY-MEMORY-PROFILE.md` + `VIVY-MEMORY-HOST-CONTRACTS.md` under
  the capability table's memory note.
- `docs/architecture/SCX-ARCHITECTURE-DESIGN.md` §C: one-line pointer to the
  frozen profile.
- `docs/superpowers/plans/memory/index.md`: Story-state column updated to the
  landed reality (MEM-0A..0E Done, MEM-1 Done, MEM-1A Ready).

## Key rulings

- Replace, don't append: the deferred MEM-1 umbrella row was rewritten in
  place and expanded into stage rows; no second stale memory entry exists in
  any tracker.
- MEM-1A's gate is now satisfied — it is the next wave — but nothing in this
  iteration schedules it; MEM-2..5 remain Blocked on their stated gates.
