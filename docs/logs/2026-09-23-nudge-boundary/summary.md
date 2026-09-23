# ND-3 — Transient nudge at the model boundary

## Goal

The next model request after a settled batch receives at most one durably
scheduled reminder (NUDGE-DESIGN §6/§7): an immutable Eino WrapModel
middleware reads run-local nudgeState, persists one `tool.nudge` event via
a Service-installed emitter, then appends a tagged `runtime_nudge` user
message to a copy of the input.

## Changes

- `internal/runtime/nudge_middleware.go` (new): `nudgeEmitter`,
  `withNudgeEmitter`/`nudgeEmitterFromContext` ctx seam,
  `newNudgeMiddleware(maxContextBytes)`, `nudgeWrappedModel` wrapping
  Generate+Stream, `trailingToolCallIDs` (moved here from the ND-0 probe —
  it is now production code keyed on the input tail).
- `internal/runtime/nudge_state.go`: `Take(ctx, ids)` now waits for the
  batch covering the input-tail ids to be registered AND sealed — the
  model can reach the boundary before the consumer registers the batch,
  so `batch == nil` alone could not distinguish "not yet durable" from
  "sealed without a reminder". Peek semantics: the same notice is
  re-served on retry with `first=false`, so the scheduling event fires
  exactly once. `nudgeNotice` gained `Status` (selects the template).
- `internal/runtime/prompt.go`: §7 fixed templates —
  `nudgeFailureTemplate`, `nudgeRefusalTemplate`, `renderNudge`.
- `internal/runtime/engine.go`: middleware registered last so its
  WrapModel sits innermost (after compaction, tool search, mount
  projection) and injects only into the final shaped input.
- `internal/runtime/service.go`: `Service.nudgeEmitter` builds the leg's
  emitter (m.build → persistAndPublish); installed in both drive and
  resumeRun legs alongside `withNudgeState`.
- `ui/src/lib/run-rows.test.ts`: regression — a soft-converted
  `tool.finished` (outcome=recoverable, result+error diagnostic) keeps
  row status `error`. RunInspector already renders `tool.nudge`
  generically (type + payload), so no new UI projection was needed.
- `sdk/internal/assembly/conformance_results.json`: refreshed the 5
  internal source digests → `27710da2…f998`.

## Contracts honored

- Exactly one `tool.nudge` journal event per scheduled reminder,
  journaled before the reminder reaches the inner model; a journal
  failure aborts the request.
- Reminder ≤1KiB and must fit remaining context budget, else
  `ErrContextBudgetExceeded`.
- No raw args/diagnostics in the notice; message carries
  kind/tool_call_id/tool_name/template_version metadata only.
- Provider retry sees the identical prepared request without a second
  scheduling event.
