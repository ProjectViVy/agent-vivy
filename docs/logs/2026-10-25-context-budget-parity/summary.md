# Context budget: one ceiling for parent and child

## What changed

A delegated child failed with

```text
sub-agent failed: The child task context exceeds the configured limit.
Cause: engine event error: [NodeRunError] runtime: context budget exceeded:
final model input requires 266407 bytes; budget is 262144
```

Two facts produced that failure, and only one of them was the number.

- **The child's budget already equals the primary's.** Child views copy the
  engine config, so `runtime.max_context_bytes` is the parent's and the
  child's limit alike. The difference is behavioural: a primary run selects and
  folds history and carries the compaction middleware, while a child view runs
  with `Compaction` disabled, so its tool loop accumulates results until the
  hard wall and then fails.
- **The number itself was too small.** The runtime estimates tokens as
  bytes/4, and `CompactionPolicy.TriggerTokens` uses
  `min(window × trigger_percent, bytes/4)`. A 256 KiB default therefore admitted
  only ~65K tokens — it clamped a 128K-token model to half its window, and a
  delegated task hit that clamp at 266 KB of accumulated context.

Changes:

- `internal/config/config.go`: `defaultMaxContextBytes` 256 KiB → 1 MiB
  (~262K tokens), so a 128K- or 256K-token window is governed by its own
  compaction trigger instead of by this ceiling. The setting stays one
  operator knob (`runtime.max_context_bytes`), read by the parent run, every
  child view, the prompt middleware, history selection and the compaction
  trigger.
- `internal/runtime/child_activation.go`: the child-only
  `maxChildActivationContextBytes = 128 KiB` ceiling is deleted. The activation
  feed (task plus pending parent messages) is bounded by the run's own engine
  budget, exactly like a primary feed, and unbounded when that budget is 0.
  This is the hidden smaller-child-limit the previous build carried.
- `config.example.yaml`: the documented value and its rationale.

## Scope

`internal/config`, `internal/runtime`, `config.example.yaml`, and this record.
No protocol, storage, UI or provider change. The child's compaction gap is not
fixed here; it is boarded in `docs/TODO.md` §0.1 as
`CHILD-CONTEXT-COMPACTION`.

## Deliberately not done

- **No per-child derivation from the model window.** The byte budget and the
  model-window trigger already meet through `TriggerTokens`; adding a second
  derivation would create a second source of truth for the same fact.
- **No child compaction.** A child deliberately starts from a clean context and
  keeps tool-result fidelity; enabling the parent's summarisation middleware
  inside a delegated task is a product decision, not a budget fix.
- **`max_tool_result_bytes` (32 KiB) is untouched.** It bounds one result, not
  the run, and remains the cheaper control for a single oversized tool output.
