# Sub-agent fault isolation — summary

## What changed

A delegated sub-agent that does not complete is now a tool result for the parent model instead of a
terminal for the whole conversation.

- `internal/tools/agent.go` — exported `ChildTaskError` plus `NewChildTaskError`, which redacts and
  bounds the child diagnostic to 2 KiB. The task guard (empty, oversized, non-object arguments) now
  returns `*tools.ArgError`, so a model mistake uses the existing `invalid_arguments` recovery.
- `internal/app/agenttool.go` — a `cancelled` or otherwise unfinished child, and a saturated child
  budget (`storage.ErrChildConcurrencyLimit`), return `*tools.ChildTaskError`. Context loss, missing
  wiring and storage/journal faults stay fatal.
- `internal/domain/tool_operation.go`, `internal/storage/tool_operations.go` — new
  `child_failed` reason allowed by the single `ValidateToolOperationOutcome` allowlist (TEXT column,
  no migration). This is what lets the durable operation commit and the journal row carry the
  classification.
- `internal/runtime/tool_failure.go` — `*tools.ChildTaskError` classifies as
  `recoverable / child_failed / effects unknown`; NUDGE-DESIGN §5 table updated.
- `internal/runtime/tooladapter.go`, `internal/runtime/service.go` — new private
  `toolInvocationError` wrapper around an unclassified invocation error, and `isProviderFailure` /
  `providerFailureMessage` reject it before any marker scan. A tool diagnostic can no longer be
  relabelled as a provider outage; genuine provider failures keep their classification.
- `internal/runtime/service.go` — child terminals now emit `child run failed` with the run id and
  cause. Previously they were silent.
- `internal/runtime/prompt_state.go`, `internal/runtime/child_sessions.go` — `PromptInput.Instruction`
  admits the engine's `childStaticInstruction` verbatim for a child run, so the durable snapshot and
  the restricted engine agree. The child no longer inherits the default web persona or the mask
  frame, and `promptMiddleware` no longer glues two instructions together.

## Scope

Kernel only. No UI/i18n change, no new `cause_category` values, no workspace-binding or model
tool-choice change, no migration.

## Explicitly not done

- `workflow` task failures still terminate the parent run (`WORKFLOW-CHILD-FAULT` on the open board).
- Hallucinated tool names, hook blocks and the unselected-tool guard remain run-fatal
  (`docs/DEFER.MD` TOOL-REFUSAL-RESIDUAL).
- No change to the provider status split introduced by the sibling `run-failure-repair` plan; the
  tool-origin exclusion is written so it can move into that plan's shared helper on rebase.

## Lane

Branch `fix/agent-tool-fault-isolation`, worktree `.worktrees/agent-tool-fault-isolation`. It shared
`internal/runtime/service.go`, `internal/runtime/tooladapter.go` and `internal/runtime/service_test.go`
with the `fix/run-failure-repair` lane; that lane landed first, so this branch merged into `stable`
after it. The tool-origin exclusion is now the first check of the landed `providerHTTPFailure`
helper, and the child branch uses the guarded `providerFailureMessage`.
