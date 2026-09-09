# Summary — UI-CHAT-ACT design slice: Journal truncation/fork proposal

## What changed

- `docs/architecture/JOURNAL-REWIND-AND-FORK.md` (new, proposal): kernel design for message editing/rewind/fork.
  Core: **logical truncation markers** (new `session_truncations` table, migration 020,
  no rows deleted) + read-time folding (same family as the compaction `foldSessionHistory` precedent, truncation composed before compaction) +
  `session/rewind` / `session/fork` two RPCs + session busy gate (`ErrSessionBusy`) + UI three-placeholder
  enablement plan. Editing = rewind + existing `turn/start` composition, with zero kernel additions.
- Foundation inventory (with file:line references landed in the document): append-only Journal + `ErrRunClosed`; the messages table
  is the model-context source of truth (run_events is the audit/subscription axis); compaction has an established "marker row + read-time folding"
  precedent; there is no per-session busy gate; child-run semantics are structurally different from fork (restart without re-execution).
- `docs/TODO.md`: UI-CHAT-ACT row updated (design slice delivered, implementation split into R1/R2/R3 + 3 public
  questions, row remains OPEN).

## What was explicitly not done

- Implementation (migration, Store, RPC, UI, e2e) — the design slice is this slice's deliverable; R1/R2/R3 are follow-up slices.
- File rewind (RB-1 axis) and physical session deletion — explicit non-goals.

## Scope

docs-only; zero code.
