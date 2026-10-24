# Agent tool promoted to the always-disclosed core — acceptance

## The model always knows it can delegate

1. Start the split pair (`just run`, then `cd ui && pnpm dev`) and open
   `http://127.0.0.1:3015`.
2. Ask a direct capability question: “你存在子代理工具吗？存在回复 1，不存在回复 0”.
3. Expected: the model answers `1` on the first turn without calling `tool_search`. Its request's
   tool schema contains `agent`.
4. Ask it to delegate a small task. Expected: it calls `agent` directly; the sub-agent runs and
   returns its answer, or a model-visible `sub-agent failed: …` result if the child does not
   complete, and the conversation continues.

## Operator control is preserved

- Setting `tools.exposure.agent: deferred` (config or settings overlay) removes `agent` from the
  first-request schema again and returns it to the tool-search catalog; `hidden` keeps it
  model-invisible. No restart of a running turn is implied — the change applies to the next turn.

## Not changed

- Child runs cannot delegate further: `agent` is filtered out of the child tool selection.
- Approvals, sandbox policy, ToolHost governance and the `agent` task limits are unchanged.
- `workflow` and other low-frequency tools remain deferred.
