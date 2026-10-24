# Agent tool promoted to the always-disclosed core — summary

## What changed

`agent` is now part of the fixed-visible tool core. The parent model sees its schema on the first
request of every run and no longer has to call `tool_search` before it can answer whether delegation
exists or use it.

- `internal/tools/toolactivation.go` — `AgentName` added to `fixedVisibleToolNames`, so
  `ResolveToolExposure` returns `direct` when the spec declares no exposure. An explicit
  `tools.exposure.agent: deferred|hidden` operator override still wins.
- `internal/tools/toolactivation_test.go` — core size 11 → 12, `agent` resolves to `direct`, and an
  explicit deferred declaration still overrides the default.

## Why

The default `tools.enabled` list already contained `agent`, but exposure (not enablement) decided
model visibility: only the fixed core was disclosed, and everything else, including `agent`, was
withheld behind Eino tool search. In practice the model could not see the delegation capability
unless it searched first, so a direct question ("do you have a subagent tool?") produced "no", and
using it depended on the model guessing a search. Delegation is a first-class product capability,
so it belongs in the same always-disclosed set as `ask_user` and the file tools.

Deferred was never an execution wall — a model that knew the name could still call `agent` — so this
change is about disclosure, not permission. Policy, approval, sandbox, and ToolHost authority are
unchanged.

## Scope

Kernel tool exposure only. No UI change, no new setting, no migration. `workflow` and the other
low-frequency tools remain deferred; `sequential_thinking`, `job_output`/`job_kill`, MCP tools and
other non-core tools keep their current exposure.

## Lane

Follow-up cut on `fix/agent-tool-fault-isolation` (worktree `.worktrees/agent-tool-fault-isolation`),
after the subagent fault-isolation work. It touches only `internal/tools/toolactivation*.go`, which
the sibling `fix/run-failure-repair` lane does not modify.
