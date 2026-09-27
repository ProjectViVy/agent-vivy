# ND-2 acceptance

Plan: `docs/plans/nudge/ND-2.md` — "One bounded run-local detector
observes typed outcomes and never loses the executed sixth call from the
Journal." Scope: state, lifecycle, Journal payloads and schema
compatibility; no model prompt injection.

## Deliverable checklist

| Plan requirement | Status | Evidence |
| --- | --- | --- |
| Move ownership of `loopWindow` into `nudgeState` | Done | `nudge_state.go` owns `window loopWindow`; mapper's `loop` field and its record call removed |
| §4 method surface on `nudgeState` | Done | `Register/MarkFailure/Failure/Complete/Seal/Take/Abort` + `withNudgeState`/`nudgeStateFromContext`; `terminalErr` added for the Service's terminal mapping until ND-3 wires the wrapper |
| `toolFailure` record type | Done | `tool_failure.go`; classifier deliberately deferred to ND-1 |
| `completedCall` outcome record; copy bounded values; no raw args beyond outstanding batch; no Journal I/O while locked | Done | `completedCall`/`nudgeBatch` hold only bounded strings; marks cleared at seal; all persistence happens in `Service.consume` outside the state's mutex |
| Mapper: metadata on `tool.finished` by call id; error non-empty for failed invocations even on nil engine error | Done | `toolResultEventsParts` stamps `outcome/reason/effects`, backfills `error` from `failure.Diagnostic` |
| Service persists every finished outcome before Seal; hard stop at settled batch boundary; sixth `tool.finished` journaled before one `run.failed` | Done | consume loop completes per appended finished; `Seal` at `remaining == 0`; `TestServiceToolLoopDetected` asserts 6 finished + 1 terminal |
| Reminders at counts 3 and 5; successful identical calls count toward stop but never remind; one notice per batch, highest threshold, tie → earliest request index | Done | `TestNudgeStateRepetitionReminders`, `TestNudgeStateSuccessCountsTowardStopOnly`, `TestNudgeStateSealEvaluatesRequestOrder` |
| `TestNudgeJournalFailure` at last result of parallel batch: no waiting handoff, no stale notice, no deadlock; only successful appends stored | Done | test asserts failed finished absent, single `run.failed`, zero `tool.nudge`, run reaches failed status |
| drive + every resume path attach fresh state; terminal paths Abort and cancel producers | Done | `drive` and `resumeRun` both build `newNudgeState()` + `withNudgeState`; consume's `defer Abort` + explicit Aborts on every exit; `runCtx` cancel unchanged |
| Strict `tool.nudge` schema (§7: five required fields, repeat_count ∈ {3,5}, template_version const, additionalProperties false) | Done | `schemas/events/payloads/tool.nudge.json`; enum entry added to `run-event.schema.json` |
| Optional outcome/reason/effects on `tool.finished`; include `parts`; preserve old payloads and required fields | Done | `tool.finished.json` extended; `required` unchanged; old rows remain schema-valid |
| Domain vocabulary tests updated only for this addition | Done | `EventTypes` 38 → 39, `domain_test.go` count updated; `EventToolMounted` slice drift left as baseline |
| Invariant errors preserve cause and abort the run | Done | `nudge_state_test.go` invariant subtests; consume propagates state errors into `emitTerminal` |
| Trajectory/run-rows keep error projection | Verified | `trajectory.go` `Error` field and `run-rows.ts` `payload.error` → `error` status unchanged |

## Global constraints

- Service.Run / Journal / policy path preserved — the detector is fed by
  the consuming Service in journal order and performs no I/O itself.
- Eino import quarantine preserved — no new `cloudwego/eino` imports
  outside `internal/runtime` (only the test file, same package).
- Native interrupts and checkpoints untouched — `errRunInterrupted` path
  only gains a state `Abort` before `handleInterrupt`.
- No automatic tool replay, no new public Port, no database migration,
  no additional model request.
- No model prompt injection — the prepared `nudgeNotice` is only
  reachable through `Take`, which has no consumer until ND-3.

## Verification gates

- Scoped test command from the plan: PASS.
- Full `./internal/runtime ./internal/domain`: PASS.
- Race on state machine + singleton-batch service paths: PASS.
- `just ci`: PASS (fmt-check, ui-ci incl. 392 vitest cases, vet,
  `go test ./...`, headless-compile, plugin-ci).
- Conformance digest re-pinned (`54da7714…`); conformance test green.

## Carryovers to later stories

- ND-1 supplies `classifyToolFailure` + `mcphost.ToolExecutionError`
  production; `MarkFailure`/`Failure` are the side channel it feeds.
- ND-3 consumes `Take` at the model boundary and must emit `tool.nudge`
  through the same persist path (payload builder `payloadToolNudge`
  exists; no emitter yet).
- The `EINO-TOOLSNODE-ERR-RACE` upstream race keeps `-race` unusable on
  parallel tool batches; still tracked in `docs/TODO.md`.
