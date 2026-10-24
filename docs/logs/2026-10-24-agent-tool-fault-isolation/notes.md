# Sub-agent fault isolation — notes

## Evidence available on this host

`AgentChildFailureKeepsParentRunAlive` reproduced the defect deterministically; the owner's live
incident left only two traces:

- `.workspace/dev/home/logs/vivy.log.2026-10-24:51,53,54` — three parent terminals: two provider
  500s with an SSE error frame (`invalid character 'd' … body: data: {"type":"error"…}`) and one
  provider 400 `tool call result does not follow tool call (2013)`. The 400 followed a user message
  persisted inside an open tool-call group (see the sibling `run-failure-repair` plan).
- No `child.*` record at all. Child terminals were silent by construction (`terminalEvent`'s child
  branch only built an event), which is why a sub-agent failure could not be attributed afterwards.

Consequence for this cut: the banner the owner saw was a **parent provider failure**, not a
tool-origin misclassification. The sub-agent fault-isolation defects are still real and independent
of that incident:

1. any `agent` error (child failed, cancelled, oversized task) was a plain Go error, so Eino's
   ToolsNode turned it into a run-fatal `NodeRunError`;
2. a tool diagnostic quoting provider wording (`API key missing`, `status code`) would have been
   classified `provider_error` by `isProviderFailure`'s substring scan;
3. the child admitted the default web persona and `promptMiddleware` glued the child directive after
   it, so the durable child instruction never equalled the restricted engine's override.

The new `child run failed` log closes the attribution gap; the E2E test closes the behavior gap.

## Why the workflow tool is not covered

`workflow` shares the JSON-RPC child machinery but defines its own failure contract (node/task
status, not one child run). Turning its errors into tool results needs a decision about which
workflow failures are model-correctable; recorded on the open board as `WORKFLOW-CHILD-FAULT`.
