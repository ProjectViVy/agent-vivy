# ORCH-06 — governed Eino Workflow execution

> **For implementers:** REQUIRED SUB-SKILL: `superpowers:executing-plans` or explicitly assigned `superpowers:subagent-driven-development`.

**Goal:** Execute validated DAG nodes as governed child Runs using native Eino Workflow with parallel dependencies and safe recovery.
**Architecture:** Runtime compiles immutable descriptor into Eino `compose.Workflow`; DAG nodes default one-shot/non-addressable and invoke ORCH-03 Service with stable idempotent node key. Host validates finite limits. Workflow output/end consumes all declared node outputs; reject unreachable/unconsumed nodes.
**Tech Stack:** Go, Eino v0.9.13 compose, Service, Journal, checkpoint bridge.
**Spec:** [architecture](../../specs/2026-09-23-issue39-eino-orchestration-design.md), R1/R4/R5/R6; [index](index.md) predecessors ORCH-03, ORCH-05.
**Review focus:** Eino controls dependency execution; no second scheduler, no duplicate side effects on resume, no budget widening, orphan/run leaks or graph checkpoint serving as a product log.

## Files and interface

NEW `internal/runtime/workflow.go`, `workflow_test.go` for Eino compiler/runner if a cohesive seam is needed. Modify `internal/runtime/{service,checkpoint}.go`, `internal/app/worker.go` or composition root wiring in `internal/app/` only as necessary. Modify storage only when ORCH-05's revision projection demonstrably lacks an atomic operation mapping. Internal proposed seam `Service.StartWorkflow(ctx, validatedRevision, operationID) (domain.RunID,error)` and `Service.WorkflowStatus(ctx, graphRunID)`; exact signatures finalized in ORCH-01/05. Node key for admission is `(graph Run ID, revision digest, node key)`; Eino node key must match validated descriptor key. No Eino imports in `internal/app`.

## Tasks

- [ ] Extend ORCH-01's integrated fixture to a production compile path. Test diamond `A → C`, `B → C` with A/B parallel start, C receiving only approved explicit outputs, no read of sibling child Sessions. Test invalid descriptor cannot reach compile, even via internal API.
- [ ] Build native `Workflow` with brokered lambda nodes; `AddDependency` for order-only edges and `AddInput` for approved mapped outputs. Use `Workflow.End()` for declared output/dependencies; `AddEnd` is deprecated. Do not use `WithMaxRunSteps`, unsupported in DAG/Workflow mode. Host enforces finite node/depth/width/output limits before compile. Reject unreachable or unconsumed nodes/outputs. Checkpoint options only as proven by ORCH-01; keep graph Run identity separate from child Runs.
- [ ] Admit each node idempotently through Service, record node-to-child mapping in Journal, wait durably for completion, project bounded output, then return it to Workflow. Test re-entry after crash before and after mapping event; brokered write runs at most once. Parallel node budget and policy come from parent; when all slots occupied, bounded backpressure or explicit failure is observable and cancellation interrupts waiting.
- [ ] Exercise approval interrupt, parent cancel, failed dependency, unreachable/unconsumed rejection, corrupted checkpoint and restart. Verify one terminal event, no post-terminal append, inspectable child Runs. Limits are Host-validated DAG bounds and Service child caps, not Eino max steps.
- [ ] Run `go test ./internal/runtime ./internal/app -run 'Workflow|Child|Approval|Checkpoint' -count=1`; both SQL backend conformance where applicable; `just ci`. Record event timelines and counts, not just eventual return values.

## Failure and rollback

If checkpoint resume reruns a node, stable idempotent admission resolves the same durable child and returns its result; unknown tool effects fail closed. If Eino parallel approval/resume is unsupported, stop public graph work and return evidence to Issue #39. Rollback is to disable graph admission and retain readable graph/child records; never rewrite execution history. No custom DAG runner fallback.
