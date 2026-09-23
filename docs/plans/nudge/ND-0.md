# ND-0: Eino Integration Contract Implementation Plan

> **For agentic workers:** Use `superpowers:executing-plans` to implement task-by-task. Use subagent-driven-development only when delegation is separately selected. This plan is not implementation authorization.

**Spec:** [NUDGE-DESIGN.md](../../architecture/NUDGE-DESIGN.md), revision ND-D1.
**Baseline:** a8d361b0244a1c40be513622bbdaebb5c9d40014.
**Status and predecessors:** [index](README.md), the sole status owner.
**Tech stack:** Go 1.26.4, Eino v0.9.13, existing Journal and tool adapters.

## Global constraints

Preserve Service.Run/Journal/policy, Eino import quarantine, native interrupts and existing budgets. No automatic tool replay, new public Port, database migration or additional model request solely for nudge. No production Journal access. Read the index's five review risks; the cases owned here are specified below. Future test code blocks are behavioral pseudocode, not compiled/passing tests.

**Goal:** Prove the pinned Eino surfaces can support the specified ordering and metadata without a second runtime.
**Architecture:** Tests use the real ADK Runner and existing Service fixtures with a scripted model and channel-controlled tools. No product recovery/nudge behavior is enabled.
**Scope:** Integration evidence and minimal test-only helpers. No production code or dependency upgrades.

## Files and interfaces

Create `internal/runtime/nudge_contract_test.go` (proposed). Reuse `newLoopGuardService`, `loopCallScript`, `waitForRunStatus`, `replayAll` from current runtime tests; extend helpers locally rather than altering unrelated fixtures. Read engine.go, service.go, mapper.go, enhanced_tooladapter.go and pinned `adk/handler.go`.

Consumes real `compose.GetToolCallID`, ordinary/enhanced adapters and `WrapModel` Generate/Stream. Produces evidence that the signatures and lifecycle in design §4/6 work, plus a fake-model input capture helper usable by downstream tests. The helper records inputs before returning a model result, and uses channels instead of sleeps.

## Task 1 — Pin correlation and model-entry order

- [ ] Confirm `go version`, `just --version`, repository runner prerequisites. Stop and report unavailable tooling without changing dependencies.
- [ ] Add a scripted fake model implementing the repository's model test interface. Capture Generate and Stream entry, model input and call IDs.
- [ ] Implement the following channel-driven scenarios against a real Eino Runner, for both ordinary and enhanced tool results:

```text
first model response requests c1 and c2 (same tool name, different arguments)
allow c2 to finish before c1
assert compose.GetToolCallID at each adapter is the exact requested ID
capture mapper tool-result events and the next wrapper entry
pause Journal append for c1
assert the proposed request barrier prevents inner model entry
release append; assert next input has exactly one result for each ID
```

- [ ] Run `go test -timeout 20m ./internal/runtime -run '^TestNudgeContract' -count=1 -v`. A baseline characterization may show inner model entry precedes durable results; that proves the need for the specified barrier, not failure of the characterization.
- [ ] Add a test-only WrapModel boundary using the state contract's completion channel; prove the wait does not prevent Eino from yielding tool results to the consumer. Test both Generate and Stream.

## Task 2 — Prove failure and shutdown behavior

- [ ] Add explicit scenarios:

```text
adapter stores recoverable metadata for c1; Eino receives nil error
mapper still associates failure metadata with c1 and never with c2
Journal append fails -> Abort releases wrapper -> inner model call count unchanged
context cancelled while waiting -> wrapper returns cancellation, no leaked wait
approval interrupt -> no conversion, resume uses new state
provider retry -> same captured reminder input, one scheduling action
```

- [ ] Inspect how a non-streamed assistant tool request is registered before dispatch; add an assertion rather than assuming streaming behavior covers it.
- [ ] Pin order of compaction versus WrapModel and budget handoff. Expected: wrapper sees final model inputs and can enforce a bounded trailing reminder; compaction-internal requests do not receive ordinary Run nudges.
- [ ] Run the focused command again and the index's race command where supported.
- [ ] Record exact Eino APIs, test names, command output and actual ordering in this iteration's verification log. Remove test-only dead code.
- [ ] Commit explicit test/log paths with `test: pin nudge Eino integration contracts`.

## Acceptance and stop conditions

Each path must show durable-tool-result-before-next-model-handoff, ID fidelity and prompt cancellation. If Eino scheduling deadlocks with the prescribed barrier, do not release ND-2 or downstream Stories: capture the failed trace and revise design §4/6 before proceeding. No alternate loop is authorized. Return a commit, test outputs, and resolved/failed design assumptions. ND-0 passing is not product acceptance.
