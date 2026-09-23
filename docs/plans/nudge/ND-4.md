# ND-4: Integrated Acceptance and Delivery Implementation Plan

> **For agentic workers:** Use `superpowers:executing-plans` to implement task-by-task. Use subagent-driven-development only when delegation is separately selected. This plan is not implementation authorization.

**Spec:** [NUDGE-DESIGN.md](../../architecture/NUDGE-DESIGN.md), revision ND-D1.
**Baseline:** a8d361b0244a1c40be513622bbdaebb5c9d40014.
**Status and predecessors:** [index](README.md), the sole status owner.
**Tech stack:** Go 1.26.4, Eino v0.9.13, existing Journal and tool adapters.

## Global constraints

Preserve Service.Run/Journal/policy, Eino import quarantine, native interrupts and existing budgets. No automatic tool replay, new public Port, database migration or additional model request solely for nudge. No production Journal access. Read the index's five review risks; the cases owned here are specified below. Future test code blocks are behavioral pseudocode, not compiled/passing tests.

**Goal:** Demonstrate the complete failure→feedback→correction/nudge→bounded-stop contract in the product path.
**Architecture:** Scripted provider fixtures verify deterministic control flow; a real split-runtime smoke demonstrates user-visible operation against isolated data.
**Scope:** Acceptance evidence and necessary regressions. New feature work returns to its owning Story.

## Files and interfaces

Create `internal/runtime/nudge_acceptance_test.go` (proposed). Update this index only from accepted evidence; update `docs/TODO.md` following the repository open/completion board rules when implementation is actually delivered. Create implementation delivery logs in `docs/logs/YYYY-MM-DD-nudge-delivery/` with summary.md, verification.md, acceptance.md; use the actual execution date. This planning log is not the implementation log.

Consumes ND-3's complete feature and all predecessor evidence. Produces checked runtime traces, mandatory CI result, smoke transcript and unresolved limitation report. No new exported interface.

## Task 1 — Cross-layer deterministic cases

- [ ] Build tests using real Service, Journal and governed tools plus fake model/provider (no network in unit tests). Use an isolated temporary directory/database and synthetic secrets.

```text
A: read missing file -> error result -> model reads an existing file -> Run completes
B: command exit 1 -> error remains inspectable -> model changes command -> completion
C: MCP IsError through ToolWorld/ToolHost/enhanced adapter -> correction in same Run
D: repeat identical unsuccessful call six times -> notices 3,5; six results; one failed terminal
E: denied write -> no filesystem mutation; refusal reminder never grants bypass
F: cancel/Journal failure while waiting -> no next model request, no deadlock
G: parallel out-of-order calls -> request-order detector, paired results, one notice per boundary
H: resume/compaction/provider retry -> no stale or duplicate scheduling
I: command with partial effect then error -> no automatic replay, effects=unknown
J: new and legacy Journal payloads -> schema validation and readable projections
```

- [ ] Run `go test -timeout 20m ./internal/runtime ./internal/mcphost ./internal/toolhost ./internal/domain -count=1`; confirm the new acceptance test names appear in verbose targeted output.
- [ ] Run the index race command on a supported toolchain host; capture limitations if unsupported, never call the race gate passed.

## Task 2 — Mandatory gate and smoke

- [ ] Run `just ci` in the repository-supported toolchain environment. Preserve exact exit code/output. Do not substitute a subset and claim CI passed.
- [ ] Launch the split product through its documented development command with explicit isolated workspace/database settings. Read existing config/bootstrap instructions to select the existing flags; never reuse data/vivy.db, data/demo or data/workspaces.
- [ ] At `http://127.0.0.1:3015`, ask for a file that is absent then provide an existing path. Observe one Run's tool failure, corrected permitted call and completion. Use a scripted provider integration for deterministic six-repeat smoke if a live model refuses to repeat; distinguish fixture evidence from live-model behavior.
- [ ] Inspect run events to confirm tool.finished.error, scheduled reminder metadata, no secret leakage and single terminal. Exercise cancel while work is waiting. Do not claim a model actually read a reminder merely from its scheduling event.
- [ ] Record actual commands, screenshots/event excerpts where useful, commit baseline, expected vs observed behavior and remaining limits (exact-match detector, reset on resume, no automatic replay).
- [ ] Run `git diff --check`, inspect feature scope and explicit file list. Update index statuses only for evidence actually accepted; update docs/TODO.md completion record only after gates pass.
- [ ] Commit evidence with `test: verify end-to-end tool failure recovery`. Submit/push through existing authorization; do not merge without authorization.

## Acceptance and stop conditions

N1–N5 each have an observed trace and required gate evidence. If CI/smoke tooling is unavailable, leave ND-4 Blocked with exact missing prerequisites, not Done. If a test reveals a contract bug, return it to ND-1/2/3; do not lower acceptance or add unrelated features. Rollback follows design §8 and retains readers for already-written events.
