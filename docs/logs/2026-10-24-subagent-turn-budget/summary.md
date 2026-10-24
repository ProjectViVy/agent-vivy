# Sub-agent turn budget

Date: 2026-10-24  Branch: `stable`  Theme: delegated work hit an eight-turn ceiling

## What the user saw

A delegated sub-agent task failed on its first call with

```
The child task reached its tool-call limit.
```

The parent then told the user "第一次调用超出了子 agent 的工具调用上限" and
suggested a lighter task. Fault isolation worked as intended (the chat stayed
alive), but a realistic delegated task could not finish.

## Root cause

Two separate caps were in play, and the tighter one was hard-coded:

- `runtime.max_tool_turns` (`internal/config/config.go`) bounded the parent at
  the default `8` model/tool cycles.
- `Engine.restrictedView` (`internal/runtime/engine.go`) forced every child to
  `maxChildToolTurns = 8` regardless of the configured value:

  ```go
  if cfg.MaxToolTurns <= 0 || cfg.MaxToolTurns > maxChildToolTurns {
      cfg.MaxToolTurns = maxChildToolTurns
  }
  ```

Because that constant was a second source of truth, raising
`runtime.max_tool_turns` would not have given a child more room either, and an
eight-cycle budget is below the read/inspect/edit cycles of a normal delegated
task.

## Change

- Children now inherit the parent's per-agent cap; `maxChildToolTurns` and its
  clamp are removed. The shared run-tree budget still bounds parent and
  children together, so no agent can widen the root account.
- `defaultMaxToolTurns` is `10000` (was `8`), owner-directed. Repeated-signature
  loop detection (VC-2) remains the runaway guard; this ceiling must not cut
  off a long but healthy task.
- The shared run-tree breakers are last-resort circuit breakers and now stay
  above the per-agent ceiling instead of preempting it: `max_run_events`
  512 → 100000, `max_model_calls` 32 → 10000, `max_run_tool_calls` 64 → 20000.
  `runtime.DefaultBudgetPolicy` (the fallback for services whose composition
  root supplies no budget) mirrors the same values.
- `config.example.yaml` now documents `max_tool_turns` and the breakers,
  including the rule that they must stay above it.

## Explicitly not done

- No separate `runtime.max_child_tool_turns` key: one per-agent cap plus the
  shared run-tree budget already cover the need.
- No change to loop detection, child depth, or concurrent-child limits.
- No UI change: the failure category and message text are untouched.
