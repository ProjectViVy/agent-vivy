# ND-2 verification

## Commands and results

| Command | Result |
| --- | --- |
| `go test -timeout 20m ./internal/runtime ./internal/domain -run 'TestNudge\|TestServiceToolLoop\|Test.*Event\|Test.*Schema' -count=1` | PASS |
| `go test -timeout 20m ./internal/runtime ./internal/domain -count=1` | PASS (all package tests, no regressions) |
| `go test -race -timeout 20m ./internal/runtime -run 'TestNudgeState\|TestServiceToolLoop\|TestLoopWindow' -count=1` | PASS, race-clean |
| `just ci` (fmt-check, ui-ci, vet, `go test -timeout 20m ./...`, headless-compile, plugin-ci) | PASS end to end |

## Race scope note

`go test -race` over a run containing a *parallel* tool batch trips the
pre-existing upstream race `EINO-TOOLSNODE-ERR-RACE` (eino v0.9.13
`compose/tool_node.go:1253`, shared `err` write in the enhanced
converter closure) — recorded in `docs/TODO.md` §0.1 since ND-0. It is
upstream module code and not fixable in-tree. The two parallel-batch
tests (`TestNudgeContract/EnhancedAdapterBatchOrderAndDurability`,
`TestNudgeJournalFailure`) exercise it; all nudge state-machine and
singleton-batch paths are race-clean. `just ci`'s `go test` does not run
under `-race` and is green.

## Test coverage added

`internal/runtime/nudge_state_test.go` — all nine ND-2 Task 1 cases:

- `TestNudgeStateRepetitionReminders`: six identical unsuccessful
  singleton batches → notices at counts 3 and 5 with correct
  call id/name/reason/template version; `errLoopDetected` at 6 via both
  `terminalErr` and `Take`.
- `TestNudgeStateSuccessCountsTowardStopOnly`: six identical successful
  calls → zero notices; hard stop still at 6.
- `TestNudgeStateCanonicalArgsIdentity`: `{"text":"spin","mode":"x"}`
  vs `{"mode":"x","text":"spin"}` decode+re-marshal to the same
  canonical args → count climbs to 3 → notice.
- `TestNudgeStateSignatureChanges`: changed result text or changed
  argument JSON resets the identical-call count — three batches each,
  no notice.
- `TestNudgeStateSealEvaluatesRequestOrder`: batch `[a5,b5]` completed
  in reverse (`b5` first) still evaluates request order; tied threshold
  selects `a5` (earliest request index).
- `TestNudgeStateInvariants`: duplicate id in one registration, empty
  batch, register overlapping an unsealed batch, completion for a
  foreign id, duplicate result id → all error; a completion with no
  outstanding batch admitted as the resume-leg singleton.
- `TestNudgeStateTakeWaitsForSeal`: `Take` blocks while results are
  outstanding and after partial completion; context cancellation and
  `Abort(cause)` release waiters; post-abort `Take` returns the cause.
- `TestNudgeStateTakeOncePerBatch`: second `Take` without a new batch
  returns nil notice.
- `TestNudgeStateFreshResumeLeg` + `TestNudgeStateContextRoundTrip`:
  fresh state has empty window/no pending notice; `withNudgeState`
  round-trips the exact pointer.

`TestNudgeJournalFailure`: a two-call parallel batch whose last
`tool.finished` append fails → run closes as `run.failed` (persisted by
`persistAndPublish`), exactly one terminal, the failed call's finished
event absent from the journal, zero `tool.nudge` rows, no hang.

`TestServiceToolLoopDetected` (updated): now asserts six `tool.finished`
events precede the single `run.failed` — the sixth outcome stays in the
journal at the hard-stop boundary.

## Adversarial checks

- Loop trip mid-batch (5th and 6th identical in one parallel batch):
  `Seal` records in request order, sets `errLoopDetected` on the sixth
  record and returns early — no notice prepared.
- `Seal` before all results are persisted sets a terminal invariant
  error rather than releasing a partial batch.
- Resume leg: `Complete` on a fresh state with no batch → implicit
  singleton; window empty (no false repetition across legs).
- Mapper `takeCompletion` fallback: resume-leg finished events without
  a parked record rebuild the outcome from the journaled payload.

## Confirmed invariants

- Ordering: `Register` runs only after the batch's `tool.requested`
  events are durable; `Complete` only after its `tool.finished` append;
  `Seal` only at a fully persisted batch → no model handoff can precede
  durability.
- Bounded retention: state copies outcome fields by value; marks are
  dropped at seal; `completions` records are popped on consumption.
- No production Journal access by the detector; the Service feeds it.
- No automatic tool replay, new public Port, DB migration, or extra
  model request.
