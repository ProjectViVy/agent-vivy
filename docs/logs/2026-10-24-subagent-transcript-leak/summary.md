# One-shot sub-agent transcripts no longer leak into the parent conversation — summary

## What changed

A one-shot sub-agent (the `agent` tool) shares its parent's session id but its transcript is
internal execution detail. The live path already suppressed it; the durable reconciler did not, so
child turns were appended to the parent session's `messages` table and then fed back to the model on
the next turn. Both paths now agree.

- `internal/runtime/message_projector.go` — `reconcileSessionMessageProjection` skips a run when
  `run.Kind == domain.RunKindChild && run.EffectiveChildMode() == domain.ChildModeOneShot` before
  projecting it. `EffectiveChildMode()` treats a legacy empty stored mode as one-shot, so older child
  rows follow the same rule. Workflow node children (`workflow_child_*`) run through
  `StartOneShotChild` with the same kind/mode/parent-session shape, so the one predicate covers them.
- `internal/storage/migrations/sqlite/048_oneshot_child_projection_cleanup.sql` and
  `internal/storage/migrations/postgres/048_oneshot_child_projection_cleanup.sql` — one statement,
  identical on both dialects, deleting already-projected rows for child runs whose `child_mode` is not
  `continuable`. The rows are derived projection and rebuild from the Journal; deleting them is a
  repair, not data loss. Forked sessions that copied leaked rows are covered because the predicate is
  keyed by `run_id`.
- `internal/storage/migrations/manifest_test.go`, `runner_test.go` — hardcoded head counts move from
  47 to 48.
- `internal/runtime/message_projector_test.go` — `TestMessageProjectorSkipsOneShotChildRuns` (parent
  and child in one session; reconcile twice; only parent rows; `ChildRunDetails` still returns the
  child summary) and `TestMessageProjectorKeepsContinuableChildSessionHistory` (a continuable child's
  own session still projects).
- `internal/storage/migrations/child_projection_cleanup_test.go` — applies `[:47]`, seeds a primary
  run, a one-shot child, a legacy empty-mode child and a continuable child with one message row each,
  runs `Apply`, and asserts only the one-shot and legacy rows are deleted.
- `docs/TODO.md` — one new open row, `UI-TERMINAL-DELTA-FOLD`, for the separate display-only defect.

## Scope

Kernel storage projection plus one migration. The Journal remains the only source of truth; the
`agent` tool result still carries the child's final answer, and Run Inspector → Children still lists
each child with its `result`/`error`.

## Explicitly not done

- **Browser display of uncommitted deltas.** A cancelled/failed run renders its trailing
  text/reasoning buffer as a finalized bubble (`ui/src/lib/run-rows.ts:329`,
  `ui/src/lib/store.ts:919`, `ui/src/lib/store.ts:437`). This never reaches `messages` — cancel emits
  no `model.completed` — so it is display-only and needs a product decision. Recorded as
  `UI-TERMINAL-DELTA-FOLD`; no UI code changed here.
- **Interrupted one-shot children.** The persisted pause variant is closed by this fix: if a child
  had already reached `model.completed`/`tool.requested`, reconcile no longer projects it.
- **Empty child workspace.** The session in the report had `workspace_path = ''`, so each run got an
  empty private scratch directory. That is the documented sandbox default (`EMPTY-WORKSPACE-SESSIONS`
  on the open board), not a projection bug.
- Continuable children are unchanged: they live in their own session and still project there.

## Eino capability check

No Eino API applies. The defect is in Vivy's durable message projection — a storage-derived cache
over the Journal — not in execution, orchestration, tool calling or context assembly. The fix reuses
the existing projector and adds no parallel runtime path.
