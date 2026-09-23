# ORCH-01 — integrated native feasibility proof

> **For implementers:** REQUIRED SUB-SKILL: use `superpowers:executing-plans` for implementation, or `superpowers:subagent-driven-development` when independent tasks are explicitly assigned. Check boxes track execution, not planning completion.

**Goal:** Determine whether Eino v0.9.13 supports Vivy's combined child and bounded workflow lifecycle without a second model loop or scheduler.
**Architecture:** Run a small `compose.Workflow` under the actual Service/broker/checkpoint boundary, with two independent child nodes and a dependent join. Validate approval interrupt, cancel and recovery through the existing stores.
**Tech Stack:** Go, Eino v0.9.13, existing runtime Service, SQLite and Postgres test adapters.
**Spec:** [architecture](../../specs/2026-09-23-issue39-eino-orchestration-design.md), gates G0/G1; [index](index.md) owns status and dependencies.
**Scope:** Proof fixture only; no production API or schema. Existing `internal/runtime/graph_conformance_test.go` proves narrower graph execution, not this combined path.

## Files and interfaces

Modify `internal/runtime/graph_conformance_test.go` only if extending its existing fixture is clearer; otherwise NEW `internal/runtime/orchestration_conformance_test.go`. Existing evidence to inspect: `internal/runtime/{service,engine,checkpoint,checkpointadapter,service_test}.go`, `internal/app/{worker,worker_test}.go`, `internal/storage/contracts.go`, `docs/eino-capability-verify.md`. No Eino import outside runtime. A test-only `node(ctx, input) (output,error)` invokes a brokered Service child seam; if no viable seam exists, document exactly which Service API or callback is missing rather than inventing a stubbed success. Expected key: one graph Run checkpoint ID distinct from two child Run IDs.

## Tasks

- [ ] Reconcile Eino pinned APIs (`Workflow.AddLambdaNode`, `AddInput`/`AddDependency`, `Compile`, `WithCheckPointStore`, `WithCheckPointID`, `WithMaxRunSteps`) and Service approval/resume behavior. Write a one-page call path and explicit identities for graph Run, node Run, tool call and checkpoint before editing. Determine which Eino runner/compose checkpoint type is actually used; ADK checkpoint support alone does not prove compose.Workflow persistence.
- [ ] Add a failing integration test for `A,B → join`: both independent nodes start before join, join receives only explicit recorded outputs, Run/Journal states reflect each, no parent personality data flows into nodes. Run `go test ./internal/runtime -run TestOrchestrationNative -count=1`; expected failure should name the missing seam, not a fake `go` error.
- [ ] Extend fixture with an approval interrupt in B, cancellation while A runs, and crash simulation with new Service/Store instances. Persist checkpoint before observed interrupt; retry the same node invocation after restart and assert the brokered write effect and child admission each happen once. Include invalid engine-version/prompt checkpoint failure. Test context cancellation propagation without an orphan active child.
- [ ] Run focused runtime and storage backend conformance plus `just ci` when feasible. Attach timestamps, Run IDs, journal sequences and checkpoint keys to evidence; classify outcome as GO, GO with a bounded API adjustment, or NO-GO, with precise Eino/Service limitation.

## Review focus, failure and handoff

The test must exercise the **production** Service authority, checkpoint bridge and write/approval broker, not a lambda that increments counters only. Explain if Eino invokes a node again on resume and how a stable operation key prevents repeating external effects. If graph checkpointing does not cover this topology, stop downstream ORCH-02–08 and update #39 with the counterexample. Do not implement a homemade executor. No data migration to roll back. Evidence is a test trace plus a technical decision recorded in `docs/logs/YYYY-MM-DD-issue39-native-proof/verification.md`; record unfinished integration as blocked in [index](index.md).
