# ORCH-06 — governed Eino Workflow execution

> **For implementers:** REQUIRED SUB-SKILL: `superpowers:executing-plans` or explicitly assigned `superpowers:subagent-driven-development`.

**Goal:** Execute validated DAG nodes as governed child Runs using native Eino Workflow with parallel dependencies and safe recovery.
**Architecture:** Runtime compiles one immutable descriptor into Eino `compose.Workflow`; node lambdas call ORCH-03's Service child invoker through a stable idempotent node key, and graph Run owns the Eino checkpoint.
**Tech Stack:** Go, Eino v0.9.13 compose, Service, Journal, checkpoint bridge.
**Spec:** [architecture](../../specs/2026-09-23-issue39-eino-orchestration-design.md), R1/R4/R5/R6; [index](index.md) predecessors ORCH-03, ORCH-05.
**Review focus:** Eino controls dependency execution; no second scheduler, no duplicate side effects on resume, no budget widening, orphan/run leaks or graph checkpoint serving as a product log.

## Files and interface

NEW `internal/runtime/workflow.go`, `workflow_test.go` for Eino compiler/runner if a cohesive seam is needed. Modify `internal/runtime/{service,checkpoint}.go`, `internal/app/worker.go` or composition root wiring in `internal/app/` only as necessary. Modify storage only when ORCH-05's revision projection demonstrably lacks an atomic operation mapping. Internal proposed seam `Service.StartWorkflow(ctx, validatedRevision, operationID) (domain.RunID,error)` and `Service.WorkflowStatus(ctx, graphRunID)`; exact signatures finalized in ORCH-01/05. Node key for admission is `(graph Run ID, revision digest, node key)`; Eino node key must match validated descriptor key. No Eino imports in `internal/app`.

## Tasks

- [ ] Extend ORCH-01's integrated fixture to a production compile path. Test diamond `A → C`, `B → C` with A/B parallel start, C receiving only approved explicit outputs, no read of sibling child Sessions. Test invalid descriptor cannot reach compile, even via internal API.
- [ ] Build native `Workflow` using `AddLambdaNode` with brokered node invocation, `AddDependency` for order-only and `AddInput` only for mapped outputs, `AddEnd`, `Compile(WithMaxRunSteps(...), WithCheckPointStore(...))` and invocation `WithCheckPointID(graphRunID...)` as verified by ORCH-01. Keep graph Run's immutable checkpoint identity separate from node child Runs. If Eino API differs in integrated fixture, use the observed minimal native API and update design/plan before implementation.
- [ ] Admit each node idempotently through Service, record node-to-child mapping in Journal, wait durably for completion, project bounded output, then return it to Workflow. Test re-entry after crash before and after mapping event; brokered write runs at most once. Parallel node budget and policy come from parent; when all slots occupied, bounded backpressure or explicit failure is observable and cancellation interrupts waiting.
- [ ] Exercise approval interrupt while sibling runs, parent cancel, failed dependency, unready descendants, corrupted checkpoint, restart after graph completion. Verify graph terminal event only once, no append after terminal, and normal child runs remain inspectable. Resource caps include graph step limit and existing per-parent child cap.
- [ ] Run `go test ./internal/runtime ./internal/app -run 'Workflow|Child|Approval|Checkpoint' -count=1`; both SQL backend conformance where applicable; `just ci`. Record event timelines and counts, not just eventual return values.

## Failure and rollback

If checkpoint resume reruns a node, stable idempotent admission resolves the same durable child and returns its result; unknown tool effects fail closed. If Eino parallel approval/resume is unsupported, stop public graph work and return evidence to Issue #39. Rollback is to disable graph admission and retain readable graph/child records; never rewrite execution history. No custom DAG runner fallback.
