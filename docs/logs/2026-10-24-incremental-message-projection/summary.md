# Incremental Journal→Message projection for live runs

Date: 2026-10-24
Branch: `stable` (root working tree)

## Why

Every tool call appeared to stall for a moment. The cause was real, not
transport or UI paint latency: `persistAndPublishEvent` projected the message
read model **before** publishing `tool.requested` and `tool.finished`, and the
projection re-read the whole run from the Journal (`Replay(runID, 0)`) and then
the whole session from `MessageStore.ListMessages` on every one of those
events. The cost grew with run and session length, and it ran twice per tool
call inside the `waitForToolsSettled` barrier that the next model leg waits on.

## What changed

- `internal/runtime/message_projector.go`
  - New `runProjectionState` cursor (`sessionID`, `seq`, `pending`,
    `noContext`) and `foldRunProjection`, which replays only events after
    `prev.seq`. `pending` carries unflushed model deltas across folds so an
    exchange can start or end between two tool calls.
  - `projectRunMessagesLocked` uses a hit path when the cursor is tracked:
    only newly derived messages are inserted through the idempotent
    `AppendMessageIfAbsent`, and `ListMessages` is skipped. A miss (first
    projection in the process, or an evicted cursor) still performs one full
    fold plus the semantic legacy-row adoption scan.
  - A failed fold returns the previous state unchanged, so deterministic
    message ids make the retry harmless and a projection conflict cannot
    advance the cursor.
  - `reconcileSessionMessageProjection` reuses the cursor and seeds it for
    live runs, so read repair no longer forces the next live projection to
    replay the run. Terminal runs are evicted; no-context runs keep a
    suppressing marker so later triggers do not replay them either.
- `internal/runtime/service.go`
  - `Service.projections` holds the per-run cursor, guarded by the existing
    `projectionMu`.
  - Cursors are dropped when the run closes (`emitTerminal`, including both
    failure branches and the deleted-session branch), when an event arrives
    for a deleted session, and when a session is deleted.

## Scope

Explicitly not done:

- No change to the event-sourced contract: the Journal is still the single
  source of truth and messages remain a derived read model.
- No tool-output streaming, transport, or UI rendering change.
- The read-repair path for *terminal* runs still replays those runs on
  `session/get` / `messages/list`; see `docs/TODO.md` `PROJECTION-READ-REPLAY`.
- The browser still folds the full event log per render (same TODO row).
