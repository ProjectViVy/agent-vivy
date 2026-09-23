# ND-3 verification

Commands run on `docs/issue58-nudge-design` (Linux, Go 1.26.8, pwsh-backed justfile).

## Scoped validation

```
go test -timeout 20m ./internal/runtime -run '^TestNudgeModelBoundary' -count=1   # ok
go test -timeout 20m ./internal/runtime -run 'TestNudge|TestToolFailure|Test.*Compaction|Test.*Approval|Test.*Resume' -count=1   # ok
```

## Coverage added

- `TestNudgeModelBoundary` — 3 identical ArgError failures → exactly one
  `tool.nudge` journaled after the batch's `tool.finished` was durable;
  the next model input ends in the tagged `runtime_nudge` user message;
  earlier requests and the next unrelated iteration carry none; tool
  results still pair ahead of the reminder.
- `TestNudgeBoundaryWaitsForDurability` — `tool.finished` journal append
  paused → inner model does not advance while the batch is unsealed;
  release → run completes.
- `TestNudgeProviderRetry` — model entry fails once and retries: both
  attempts' inputs are DeepEqual (reminder included), one `tool.nudge`.
- `TestNudgeRefusalTemplate` — 3 identical denied calls schedule the
  refusal template ("Respect the policy or user decision… Do not bypass");
  the denied tool never executed.
- `TestNudgeSchedulingPersistFailure` — journal fail on `tool.nudge`
  aborts the run (no unrecorded injection).
- `TestNudgePrepareCopiesInput` — input slice not mutated; tagged
  trailing reminder; retry re-serves the notice without re-emitting.
- `TestNudgePrepareSkipsWithoutResults` — no trailing tool results →
  delegated unchanged.
- `TestNudgePrepareBudget` — tight budget → `ErrContextBudgetExceeded`.
- `TestNudgeStreamMatchesGenerate` — Stream takes the same barrier.
- `TestNudgeSpoofedResultIgnored` — reminder-shaped tool output produces
  no notice and no injected message.
- `TestRenderNudgeTemplates` — status-selected templates, ≤1KiB.
- `TestNudgeStateTakeOncePerBatch` (updated) — same notice re-served with
  `first=false`.
- `ui/src/lib/run-rows.test.ts` — soft-converted `tool.finished` keeps
  row status `error` via `payload.error`.

## Full gate

`just ci` — PASS (fmt-check, ui-ci incl. 393 vitest cases, vet,
`go test ./...` incl. conformance digest `27710da2…f998`, headless-compile,
plugin-ci).

## Deliberate deviations from ND-3.md

- `newNudgeMiddleware` takes `maxContextBytes int` (plan showed no args):
  the budget check inside prepare needs the run's context byte budget,
  which is engine config, not middleware-discoverable state.
- `Take` signature became `Take(ctx, ids)` returning
  `(*nudgeNotice, bool, error)` — required by §7 retry semantics and by
  the registration race (contract `Await` parity, proven by ND-0).
