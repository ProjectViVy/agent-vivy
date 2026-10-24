# MR-4: child-run recovery context and terminal events

Defects: D9, D10.

## Steps

1. `internal/runtime/child_recovery.go` `rebuildPendingChild`: create
   `runCtx, cancelRun := context.WithCancel(context.Background())`, register
   `s.active[run.ID] = cancelRun`, and populate `pendingRun{runCtx: runCtx}`
   — a nil parent context panics at resume (`service.go`).
2. `internal/runtime/service.go` resume-failure branch: emit via
   `s.emitTerminal(ctx, m, s.terminalEvent(ctx, m, err))` so a child run
   produces `child.failed`/`child.cancelled`, never `run.failed`.

## Evidence

`TestServiceChildMailbox*`, `TestParentRunReceivesChildReply*`, child resume
tests green.
