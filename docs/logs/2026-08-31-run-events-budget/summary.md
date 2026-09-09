# 2026-08-31 — Exempt streaming deltas from the run-event budget (TT-4)

## What changed

Real-path smoke testing after the two-tier tools landed exposed the issue:
a request with tools repeatedly triggered
`run budget circuit breaker kind=events limit=512`. The cause was that
`internal/runtime/mapper.go` emitted one `model.delta` for every streaming
chunk (`model.reasoning_delta` is also emitted in reasoning mode), while
`reserveMappedBudget` in `internal/runtime/service.go` counted one budget
event for every mapped event. A normal-length reply (hundreds of chunks)
could therefore exhaust MaxEvents=512 by itself, and multiple rounds of tool
calls inevitably tripped the breaker midway, appearing as the literal
"This conversation did not complete…reached a safety budget".

Fix: `reserveMappedBudget` skips event accounting for streaming chunk events
(`EventModelDelta` / `EventModelReasoningDelta`). Runaway protection is
unchanged—`model_calls` (one charge per generation, limit 32) and
`tool_calls` (limit 64) remain the actual runaway gates; delta payloads are
still clamped by the mapper to max_event_payload_bytes.

Same-day cleanup: removed the two leftover
`tools.enabled: [echo_info, write_note]` lines from the host root's
`config.yaml` (a gitignored local file), restoring the code-default 26-tool
surface—the direct reason for the literal model response "the model only reports
three tools: echo_info/write_note/skill". The `tools_enabled` override in settings.yaml
was cleared through the Settings action `Restore configuration defaults`; that override had previously been used to verify the save path,
and the path was confirmed usable.

## What was explicitly not done

- Do not change delta's Journal persistence contract (each chunk remains a
  persistent event for feed replay); only exempt it from budget accounting.
- Do not tune the MaxEvents value—512 is sufficient for semantic events;
  tuning it would mask the issue.
- Do not address the fact that empty-content chunks still create empty delta
  events (Journal noise, outside this topic).

## Related

- `docs/TODO.md` §0.1 TT-4 (closed in this iteration)
- `docs/logs/2026-08-31-two-tier-tools/` (the preceding iteration that exposed this issue)
