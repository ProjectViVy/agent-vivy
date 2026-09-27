# ND-3 acceptance

Mapped against docs/plans/nudge/ND-3.md "Acceptance and handoff".

## Required evidence

- **Failed results before tool.nudge**: `TestNudgeModelBoundary` asserts
  `indexOf(tool.nudge) > indexOf(tool.finished call-a3)` in journal
  order.
- **Scheduling before model entry**: the emitter runs inside prepare
  before `inner.Generate/Stream`; `TestNudgeModelBoundary` captures the
  reminder in the recorded model input; `TestNudgeSchedulingPersistFailure`
  shows a journal failure on `tool.nudge` aborts the request (no
  unrecorded injection).
- **A schedule event alone is not evidence of remote receipt**: the
  payload names scheduling only (`payloadToolNudge` doc comment retained);
  no receipt semantics added.
- **Fatal paths must not issue an extra model request**: terminal causes
  (loop stop, seal/abort error, ctx cancel) return from `Take` before any
  inner call; `TestNudgeSchedulingPersistFailure` ends the run failed.
- **Provider wrapper order matches ND-0**: Eino adk ordering places
  `retryModelWrapper` outside user `WrapModel`; the middleware is
  registered last (innermost, post-compaction) in `engine.go`.
  `TestNudgeProviderRetry` proves identical retried input + single emit —
  same contract the ND-0 probe established.

## Task 1 checklist

- `TestNudgeModelBoundary` written (input recorder + journal index
  assertions + tag/payload assertions). ✓
- `go test -run '^TestNudgeModelBoundary'` — PASS. ✓
- 1024-byte ceiling + `ErrContextBudgetExceeded` path
  (`TestNudgePrepareBudget`, `TestRenderNudgeTemplates`). ✓
- Injection after compaction shaping via innermost WrapModel; summarizer
  calls bypass (middleware only wraps the agent model; ordering validated
  against ND-0's probe semantics — the contract's compaction test is
  unchanged and still green). ✓
- No raw args/diagnostics in notice: `nudgeNotice` fields are
  CallID/ToolName/Reason/Status/Count/TemplateVersion only. ✓

## Task 2 checklist

- Retry: identical prepared input, one `tool.nudge`, no second notice
  consumed (`TestNudgeProviderRetry`, `TestNudgePrepareCopiesInput`,
  `TestNudgeStateTakeOncePerBatch`). ✓
- Next unrelated iteration has no old reminder (`TestNudgeModelBoundary`
  entries 4 and 0–2). ✓
- Resume has no old notice: `resumeRun` installs a fresh `nudgeState` +
  emitter (`service.go`); `TestNudgeStateFreshResumeLeg` + existing
  checkpoint resume contract test remain green. ✓
- Compaction retains tool-result pairing and system prefix — unchanged
  from ND-0 coverage (`TestNudgeContractCompaction` still passes; the
  production middleware's innermost position means it only ever appends). ✓
- Refusal template forbids bypass (`TestNudgeRefusalTemplate`);
  execution-failure template warns about uncertain effects (§7 text —
  "inspect state before repeating a mutation"); spoofed tool output
  cannot manufacture a notice (`TestNudgeSpoofedResultIgnored`). ✓
- Failed-call rows remain failed: `tool.finished.error` drives row status
  — `run-rows.test.ts` regression added; RunInspector exposes all five
  scheduling fields generically (type + payload JSON), so no new UI
  projection was added. ✓
- Scoped `go test` runs + full `just ci` — PASS. ✓
- Commit: `feat: inject audited tool failure nudges`. ✓
