# ND-1: Selective Tool Failure Feedback Implementation Plan

> **For agentic workers:** Use `superpowers:executing-plans` to implement task-by-task. Use subagent-driven-development only when delegation is separately selected. This plan is not implementation authorization.

**Spec:** [NUDGE-DESIGN.md](../../architecture/NUDGE-DESIGN.md), revision ND-D1.
**Baseline:** a8d361b0244a1c40be513622bbdaebb5c9d40014.
**Status and predecessors:** [index](README.md), the sole status owner.
**Tech stack:** Go 1.26.4, Eino v0.9.13, existing Journal and tool adapters.

## Global constraints

Preserve Service.Run/Journal/policy, Eino import quarantine, native interrupts and existing budgets. No automatic tool replay, new public Port, database migration or additional model request solely for nudge. No production Journal access. Read the index's five review risks; the cases owned here are specified below. Future test code blocks are behavioral pseudocode, not compiled/passing tests.

**Goal:** A supported failed invocation becomes an unsuccessful tool result that allows same-Run correction.
**Architecture:** Classify at the common governed adapter and mark per-call failure metadata; retain native fatal/interrupt paths. Carry first-party MCP IsError across the existing internal error channel.
**Scope:** Failure producers and adapter result framing; detector/Journal integration belongs to ND-2.

## Files and interfaces

Modify `internal/runtime/tool_failure.go` (types supplied by ND-2); create `internal/runtime/tool_failure_test.go`, `internal/mcphost/tool_error.go` (proposed). Modify `internal/runtime/tooladapter.go`, `internal/mcphost/toolworld.go`; add integration assertions to `internal/runtime/enhanced_tooladapter_test.go` and `internal/mcphost/toolworld_test.go` (create test file if absent). Read `internal/app/assembly_tools.go`, `internal/toolhost/host.go`, `internal/tools/command.go`, `internal/runtime/command_backend.go` without unrelated changes.

Consumes accepted ND-2 toolFailure/nudgeState and ND-0 correlation evidence. Produces classifyToolFailure and mcphost.ToolExecutionError exactly as design §4; use ND-2 MarkFailure for the real per-call metadata path. No separate collector.

## Task 1 — Classify at the proven origin

- [ ] Write table-driven `TestToolFailureClassification` covering all rows of design §5, with wrapped errors and secret-bearing diagnostics.

```text
invocation ArgError -> recoverable invalid_arguments
readonly invocation wraps fs.ErrNotExist -> recoverable not_found
write invocation wraps fs.ErrNotExist -> original fatal error
parent ctx cancelled plus ArgError -> original cancellation
unknown error or storage failure -> same error cause, no soft conversion
native approval interrupt -> unchanged interrupt behavior
```

- [ ] Run `go test -timeout 20m ./internal/runtime -run '^TestToolFailure' -count=1`; observe failures before changing behavior.
- [ ] Implement classifier precedence: parent cancellation; caller preserves interrupt; exact typed allowlist at invocation origin; redact + compact diagnostic; otherwise original error. Never classify a dispatch/hook error using the invocation-only allowlist.
- [ ] Modify InvokableRun/invoke only enough to preserve origin, convert eligible failures and publish ID metadata. Keep the normal successful-result path identical. Refusal sites assign exact reason/status; human-denial return sites also mark failure.

```text
result, err := governed execution
if native lifecycle error: return original error
if known refusal: retain refusal result and mark refused
if typed invocation failure: mark failure; return bounded untrusted diagnostic, nil
otherwise: preserve result/error
```

## Task 2 — Structured failures without SDK changes

- [ ] For reserved bash/execute results, decode the actual CommandResult before framing/truncation; nonzero exit marks command_failed with effects=unknown. Keep stdout/stderr/exit_code available. Do not reinterpret shell-service errors globally.
- [ ] Change ToolWorld.Invoke's IsError path to return mcphost.ToolExecutionError containing the original remote text. Keep JSON-RPC/transport errors distinct. Test through Host.Invoke and governedTool, not only the constructor.
- [ ] Verify enhanced normalization returns valid parts and no raw envelope corruption; failure identity is metadata, never a string-prefix heuristic.
- [ ] Add `TestToolFailureCorrection`: scripted model observes invalid_args/not_found/command_failed/remote_tool_error, corrects its next action and completes the same Run. Every effectful action still traverses policy.
- [ ] Add refusal test proving denied writes never execute and the feedback does not grant alternative bypass authority. Add unknown-effect write test proving invocation count is one unless the model explicitly requests a further permitted action.
- [ ] Run `go test -timeout 20m ./internal/runtime ./internal/mcphost ./internal/toolhost ./internal/app -run 'TestToolFailure|TestMCP|TestEnhanced|TestToolAdapter' -count=1` and check test output confirms each newly added test ran.
- [ ] Commit explicit paths and evidence with `feat: preserve recoverable tool failure feedback`.

## Acceptance and handoff

Return failure table, direct/enhanced/MCP traces and error-chain/redaction evidence. ND-3 consumes the integrated typed metadata, not a parsing convention. Stop if preserving MCP results requires a public SDK change; the current design intentionally avoids one. Unknown failures remain fatal and local timeout recovery is not implicitly added.
