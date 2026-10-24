# Tool results replayed in call order

## Problem

A turn can hold more than one tool call in one assistant message. The Journal
records `tool.finished` when each call completes, not when it was requested,
so an approval-gated call (for example `bash` waiting on approval) is persisted
after a later call (`agent`) that finished first.

Every later turn rebuilds the model feed from the Journal. That feed grouped
the results by completion order, so the provider saw the second call's result
immediately after the first call. Strict providers reject that feed:
MiniMax returns `400 invalid params, tool call result does not follow tool
call (2013)`. Because the feed is rebuilt on every attempt, retrying the same
run reproduced the same 400 — the failure was not transient.

## Change

`stabilizeToolTurns` in `internal/runtime/context.go` (previously
`pairToolTurns`) now buffers tool results per open call group and emits them in
call order before closing the group. Behaviour kept from the previous version:

- a user row written inside an open group (a mid-run steer) is deferred until
  the group's results are emitted;
- results without a matching call are still emitted;
- calls without a result are still dropped.

The Journal write path is unchanged: live events keep their completion order,
and only the model projection is reordered.

## Scope

- `internal/runtime/context.go`, `internal/runtime/context_test.go`.
- No wire, storage, migration, UI or policy change.

## Explicitly not done

- No provider transport retry. A 400 from a malformed feed stays terminal.
- No change to the intermittent child first-call `400 (2013)` reported before a
  tool group exists; that needs request sampling and is tracked separately.
  Superseded 2026-10-25: that rejection now gets one bounded retry of the
  identical payload, see
  `docs/logs/2026-10-25-tool-result-order-retry/summary.md`. The root cause on
  the provider side is still unconfirmed.
- `TOOL-REFUSAL-RESIDUAL`, `WORKFLOW-CHILD-FAULT` and `EMPTY-WORKSPACE-SESSIONS`
  are unaffected.
