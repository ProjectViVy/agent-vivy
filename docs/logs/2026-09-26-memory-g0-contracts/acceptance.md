# Acceptance: how a human can tell the memory track state is real

Product/maintainer view: the repo trackers now say what is true — the BML
library exists as `bml/`, the G0 contracts are frozen docs, and the next
actionable story is MEM-1A.

## 1 — Trackers agree with reality

```text
grep -n 'MEM-' docs/TODO.md docs/DEFER.MD docs/research/OPEN-ITEMS.md
```

- `docs/TODO.md` §0.1 shows MEM-0A..0E DONE with their doc paths, MEM-1 DONE
  with `bml/` + PR #62, MEM-1A READY-NEXT, MEM-2..5 BLOCKED.
- `docs/DEFER.MD` shows MEM-1 as PARTIALLY LANDED (wiring + later stages
  deferred) and `MEM-CAP` carrying the still-deferred AutoDream / Evolution /
  RAG scope.
- `docs/research/OPEN-ITEMS.md` mirrors both.
- `docs/superpowers/plans/memory/index.md` is the authoritative story table
  and says the same.

## 2 — The cited artifacts are real

Open `bml/` (Go module with `go.mod`, `provider.go`, `README.md`) and the four
G0 docs; each exists on `feat/memory` and cites its sources. MEM-1's own
iteration log is `docs/logs/2026-09-26-memory-bml/notes.md` — this directory
covers the G0 set and the foldback, not the library work.

## 3 — SCX docs point at the frozen contracts

`SCX-PLUGIN-INTEGRATION.md` names both frozen contract docs under the
capability table; `SCX-ARCHITECTURE-DESIGN.md` §C names the profile. One line
each — the contracts live in `docs/architecture/`, not in the SCX text.

## Known limits

- MEM-1A is unblocked (Ready) but not scheduled by this iteration; its plan
  still needs to be written.
- MEM-2..5 stay Blocked on the gates named in the index; no provider,
  governance, or Garden work is implied.
- The deferred `MEM-CAP` capabilities (AutoDream / Evolution / RAG) remain
  deferred and still need their own capability proposal.
