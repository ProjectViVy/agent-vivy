# VC-2: `agent` subagent tool (D5)

# Date: 2026-09-01 | Branch: `feat/vc1a-bash-tool` (worktree `agent-vivy-vc0`)

## Overview

Implement the TODO VC-2 `agent` subagent tool (D5 ruling: visible to the model,
mapped onto the existing child-run machinery; a subagent can wear a mask, has no
kernel, and has a clean context, rather than being a Crush-style coordinator or
named agent).

The implementation has three parts:

1. **`agent` tool in the tools layer** (`internal/tools/agent.go`)
   - Parameters are `task` (required, a complete self-contained delegation
     brief) + `mask` (optional persona hint); task is capped at 64 KiB and mask at
     2 KiB, with empty/oversized values rejected.
   - `Readonly: true`—the subagent reuses the same automatic-approval rules and
     cannot produce approval-requiring effects internally (see the narrowed tool
     surface).
   - The description states: read-only tools, no shell, no MCP, no access to the
     main conversation, and no ability to spawn another subagent.
   - The app layer implements the `AgentOperations` seam; `BuiltinWithAgent`
     registers the tool only when an implementation is supplied (the default
     `BuiltinWithWeb` does not include it).
2. **App-layer assembly** (`internal/app/agenttool.go`)
   - `agentToolRef` is late-bound: the builtin registry is built before
     `workerManager` (the app.go ordering constraint), so an empty reference is
     created first and `arm()` runs after the manager is ready.
   - `StartAgentTask`: obtain the parent run ID from the run-scoped context →
     `StartChild` (`Text=task`, `System=persona hint`, `ToolNames=read-only
     subset`) → synchronously wait for the terminal state with `WaitChild`; when
     the parent context dies (cancel/shutdown), call
     `CancelChild(context.WithoutCancel(ctx), …)` so no worker process hangs.
   - `readOnlyToolNames`: the subset of registry entries with `spec.Readonly`,
     excluding the `mcp_` prefix surface and `agent` itself—the subagent cannot
     nest or touch MCP.
   - `agentSystemPrompt`: fixed base (one-shot task, clean context, return the
     final message verbatim) + optional `Persona hint (mask)` line. The mask is a
     hint, not a named agent—there is no kernel or persistent identity.
3. **System prompt seam** (threaded through the worker protocol)
   - `ChildRequest.System` (RPC) → `driveChild` → `worker.Spec.System` →
     `RunRequest.System` → the first `system` message in turnLoop. The model
     broker already maps the system role, so no change was needed. On the worker
     side, System receives the same `maxChildTextBytes` (64 KiB) cap as Text.

**Governance semantics (reuse the existing child-run machinery, with no new
mechanism)**: approval = `ApprovalKindChild` joined to the parent session
(visible in Review Center); budget = nested `BudgetLedger.Child` (the subagent
burns the parent run's quota and cannot reset the circuit breaker); cost
aggregation = `legacyModelBroker` records `model.usage` after `model/complete`
returns (the existing D9 path), so session-level token/cost statistics
automatically include subagent usage.

**Configuration**: add `agent` to the default `tools.enabled` list in
`config.example.yaml` and `internal/config`.

## Relationship to Crush (FSL-1.1-MIT compliance)

Crush's `task` tool is the behavioral reference (read-only subagent delegation);
no code was copied into this project, and the implementation is entirely built
on Vivy's existing child-run machinery. Per the D5 ruling, the persona model has
no coordinator and no named-agent registry; mask is a one-time hint.

## Explicitly not done

- Depth/concurrency limit changes (`maxChildDepth=4`, `maxChildrenPerParent=4`,
  subagent `childMaxTurns=8`).
- Real-time presentation of the subagent event stream in the parent session (only
  the terminal result returns; events are in Journal).
- Editing the mask permission surface (D5 mentioned "editable like diva"—a later
  UI item).
- Higher-level subagent capabilities such as agentic_fetch (WEB-1).
