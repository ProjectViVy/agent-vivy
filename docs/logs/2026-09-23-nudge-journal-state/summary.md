# ND-2 — bounded run-local nudge detector over durable outcomes

Iteration: `docs/plans/nudge/ND-2.md`, issue #58 (agent error correction).

## What changed

One run-local detector, `nudgeState` (`internal/runtime/nudge_state.go`), now owns
the repetition window and the batch lifecycle:

- `nudgeState` holds at most one outstanding batch (`ids` in request order,
  `remaining`, correlated `calls`), the typed-failure side channel
  (`marks`), the moved `loopWindow`, one prepared `nudgeNotice`, and the
  terminal cause. Methods per NUDGE-DESIGN §4: `Register`, `MarkFailure`,
  `Failure`, `Complete`, `Seal`, `Take`, `Abort`, plus `terminalErr` for
  the consuming Service.
- `loopWindow.record` now returns the signature count beside
  `errLoopDetected` so Seal can build notices at counts 3 and 5 and stop
  the run at 6 (`internal/runtime/loopguard.go`).
- The mapper no longer records into the window. On a tool result it
  resolves the typed failure mark by call ID, stamps optional
  `outcome`/`reason`/`effects` onto `tool.finished`, backfills `error`
  with the bounded diagnostic when Eino delivered nil, and parks a
  `completedCall` record until durability (`internal/runtime/mapper.go`).
- `Service.consume` drives the detector strictly in journal order:
  `Register` after the turn's `tool.requested` batch is durable,
  `Complete` per appended `tool.finished`, `Seal(nil)` only when the
  batch is fully persisted. A seal-detected loop, an invariant error, a
  persist failure, an interrupt, or a terminal all `Abort` the state so
  no boundary can wait on a dead leg. Both `drive` and `resumeRun`
  attach a fresh state per leg via `withNudgeState` (`internal/runtime/service.go`).
- `toolFailure` landed as the §4 record type
  (`internal/runtime/tool_failure.go`); the classifier itself is ND-1.
- New event `tool.nudge` added to the domain vocabulary (38 → 39) with a
  strict `schemas/events/payloads/tool.nudge.json` (all five fields
  required, `repeat_count` in {3,5}, `template_version` = `nudge-v1`,
  `additionalProperties: false`) and the `run-event` enum entry after
  `tool.mounted`. `tool.finished.json` gained optional `parts`,
  `outcome`, `reason`, `effects`; required fields unchanged.

## Semantics locked in

- Successful identical calls feed the window and the hard stop at 6 but
  never produce a reminder — only marked failures are notice candidates.
- One notice per sealed batch: highest threshold wins, ties resolve to
  the earliest request index. `Take` is blocking but consumes at most
  once per batch; a second `Take` with no new batch returns nil.
- Completion order does not matter: `Seal` iterates `batch.ids` (request
  order). Out-of-order completions inside a batch are evaluated in
  request order.
- A completion with no outstanding batch is admitted as an implicit
  singleton — this is how the decided call of a resume leg reaches the
  detector without a matching `tool.requested`.
- The sixth identical call now lands its `tool.finished` in the journal
  before `run.failed`: the hard-stop decision moved from event
  construction to the settled batch boundary. The old "five finished"
  assertion is now six, per plan.
- Unregistered/duplicate/overlapping batch operations return invariant
  errors that abort the run; `Seal` before full persistence sets the
  terminal cause for the same reason.

## Deviations and notes

- `tool.finished.json` previously omitted `parts` while the mapper has
  emitted it for multimodal results; the schema now declares it, closing
  the noted mismatch where fixtures exercise it.
- `EventToolMounted` was already absent from the `EventTypes` slice
  (baseline drift); left untouched per "vocabulary tests only for this
  addition".
- `TestNudgeJournalFailure` necessarily uses a two-call parallel batch;
  under `-race` it trips the already-documented upstream race
  `EINO-TOOLSNODE-ERR-RACE` (`compose/tool_node.go:1253`), not Vivy code.
  Race verification covers the state machine and singleton-batch paths.
- Conformance `sourceSha256` re-pinned for the new/changed `internal/`
  files (`54da7714…`), same procedure as ND-0.
