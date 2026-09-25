# ORCH-03 — native governed child execution

> **For implementers:** REQUIRED SUB-SKILL: `superpowers:executing-plans` or explicitly assigned `superpowers:subagent-driven-development`.

**Goal:** Route child model/tool execution through the same Service/Eino path as the parent, retaining child authority and existing Run control behavior.
**Architecture:** An internal Service activation consumes ORCH-02's durable binding; Eino runner handles model/tool turns, while Service owns Run, approval, event and budget edges. Origin parent Run is immutable lineage; current authorizer Run is recorded per activation.
**Tech Stack:** Go, Eino ADK/current Engine, host policy, checkpoint bridge.
**Spec:** [architecture](../../specs/2026-09-23-issue39-eino-orchestration-design.md), G1; [index](index.md) predecessor ORCH-02.
**Review focus:** eliminate second model/tool loop without moving Eino imports into App; ensure policy and approval parity.

## Files and interface

Modify `internal/runtime/{service,engine,checkpoint}.go` as required by G0, `internal/app/{worker,agenttool}.go`, existing `internal/runtime/service_test.go`, `internal/app/worker_test.go`; NEW `internal/runtime/child_activation.go` only if Service becomes clearer. Inspect `internal/worker/` before removal; delete obsolete execution loop and broker wrappers only when all consumers and logs migrate. Internal proposed seam: `Service.ActivateChild(ctx, binding, task, toolSelection, maskHint) (domain.RunID,error)`, returning the ORCH-02-admitted ID and using `RunWithOptions` or the same internal drive path. This is an internal contract sketch; G0 may reveal a smaller signature. Keep `agentToolRef.StartAgentTask(ctx, task, mask)` compatibility while it delegates to this seam.

## Tasks

- [ ] Capture red tests for the present execution: child uses actual Eino path, stays read-only by default, preserves child max turns/depth/slot/budget, writes child Session messages only, emits prompt snapshot, and never sees parent personality. `go test ./internal/app ./internal/runtime -run 'Child|Worker|Prompt' -count=1` should fail for the missing native path, not on absent tooling.
- [ ] Reuse Service admission, parent model, parent-bounded tool selection, policy and approval handling. Convert worker manager into lifecycle glue. Parent Session deletion fences child. Continuation requires a new active Run in same original parent Session and intersects authority with original ceiling. Context is explicit task, admitted direct mail and approved dependency outputs only, no transcript/personality/history.
- [ ] Verify checkpoint-at-interrupt before outward approval event, same prompt/version on resume, refused tools and denied approval as existing Service semantics. Use stable tool call/effect IDs on restart. Integrate the G0 proof's missing Service seam only if its evidence demonstrates necessity.
- [ ] Remove the now-dead handwritten model/tool loop after comparing all former event/usage/approval messages and callers. Do not leave two selectable production engines or silently change output contracts.
- [ ] Run targeted tests including `TestChildToolBrokerApprovalResumesAndExecutes`, `TestChildStartRejectsDepthAndConcurrencyBeforeSpawning`, `TestDeleteSessionCancelsLiveChildWorker` (rename fixtures to actual native behavior), `go test ./internal/app ./internal/runtime -count=1`, then `just ci`.

## Failure and rollback

Child crashes are terminal or resumable strictly under Service state machine and verified checkpoint; never silently replay an unknown tool. Preserve old RPC and storage read compatibility during cutover. If G1 parity fails, keep native child surface gated and restore previous safe behavior through a focused revert; do not expose experimental mode to users. Record before/after Run/Journal traces and a production-path approval test; unblock ORCH-04/06 only with these artifacts.
