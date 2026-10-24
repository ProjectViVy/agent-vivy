# Sub-agent lifecycle audit: a silent child answer and the untested paths

Date: 2026-10-24
Scope: Vivy kernel (`internal/app`, tests only elsewhere), reference product
`.workspace/pi`.

## What changed

1. **An empty child answer is now a model-visible completion note.**
   `StartAgentTask` (`internal/app/agenttool.go`) returned `final.Result`
   verbatim on `completed`, so a child that finished without writing text
   produced a tool result containing only the untrusted-output header. The
   parent received no usable outcome. It now returns
   `"the sub-agent finished without a written answer"`. The reference
   subagent tool applies the same rule (`getFinalOutput(...) || "(no output)"`
   in `.workspace/pi/packages/coding-agent/examples/extensions/subagent/index.ts`).

2. **New deterministic lifecycle suite** (`internal/app/agent_lifecycle_e2e_test.go`,
   five E2E tests over the scripted OpenAI-compatible provider):
   - two delegations in one assistant message both return their own answer
     (tool-call-id routing);
   - a parent cancel stops the running child and its in-flight provider call
     (no orphan child, child terminal is `cancelled`);
   - a child that exhausts the per-run turn ceiling is a recoverable
     `child_failed` tool result, not a parent failure;
   - an oversized child answer is truncated at the child-result budget with
     the `[child result truncated]` marker;
   - an empty child answer is model-visible.

## Audit findings that needed no change

- **Parent cancel → child cancel** already works: the agent tool cancels the
  child from a detached context when its own run context ends. Now pinned by
  a test that also asserts the child's provider request is aborted.
- **Child slot count**: `MaxActiveChildrenPerRun = 4` matches the reference
  default (`MAX_CONCURRENCY = 4`). Vivy executes a turn's tool calls
  sequentially (`compose.ToolsNodeConfig.ExecuteSequentially` in
  `internal/runtime/engine.go`, required by the approval-fencing design), so
  a parent turn holds at most one one-shot child at a time and the slot cap
  is not reachable from model-driven delegation. The reference queues behind
  the same number. Left unchanged; the existing `ErrChildConcurrencyLimit`
  mapping to `ChildTaskError` remains the graceful path for the
  host-driven continuable-child surface.
- **Thinking mode inheritance**: the child run context is derived with
  `context.WithoutCancel(ctx)` from the parent run's tool context, so the
  run-scoped thinking mode is preserved without explicit plumbing.
- **Read-only child surface and task/result bounds** (64 KiB task, 32 KiB
  answer, 2 KiB failure detail) are already correct and documented.
- **Duplicate delegation after a crash**: children are read-only and a
  worker lost to restart fails closed with an explicit message, so replay
  cannot double-apply side effects; a retry re-delegates, which the
  reference also treats as safe for read-only work.

## Explicitly not done

- No change to sequential tool execution or the child slot constant.
- No change to the continuable-child RPC surface or the UI.
- Linux `just wiring-ci` / `just kernel-ci` and the split-pair browser smoke
  were not run on this host; see `verification.md`.
