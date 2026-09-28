# MR-3: tool-set union and durable-op transparency

Defects: D7, D8. The durable tool-operation coordinator (DAG side) intercepted
tool invocation before the refusal/interrupt semantics (goal-plan side) could
run.

## Steps

1. `internal/config/config.go`: restore the union default for
   `Tools.Enabled` (`agent`, `enter_plan_mode`, `submit_plan`, `get_goal`,
   `create_goal`, `report_goal`, `workflow`, `reply_parent`, `child_inbox`).
2. `internal/runtime/tool_operation.go`:
   - `compose.IsInterruptRerunError(invokeErr)` → `finish("", invokeErr)`
     (interrupts are suspension signals, never operation outcomes);
   - `asToolRefusal(invokeErr)` → record `refusalToolResult(name, reason)` as
     the durable `Result` (failure stays empty) AND return the typed refusal
     so `InvokableRun` still runs `markInvocationFailure` and the
     `policy.evaluated` journal;
   - `isModelWorkTool` exempts the five work-control tools from op admission
     (`CommitWork` dedups by `modelWorkIdentity` already).
3. `internal/runtime/tooladapter.go`: skip `Lookup`/`Admit` for
   `isModelWorkTool(spec.Name)`.

## Evidence

`TestPlanGoalIntegratedReadOnlyDeniesPlanWrite`, the interrupt/approval matrix,
`goal_armed`/stale-goal refusal tests, and `TestModelWorkIdentity` green;
`internal/runtime` 19 → 0 failures.
