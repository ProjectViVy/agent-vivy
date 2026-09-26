# ORCH-01 — recovery foundation and integrated native proof

> **For implementers:** REQUIRED SUB-SKILL: use `superpowers:executing-plans` for implementation, or `superpowers:subagent-driven-development` when independent tasks are explicitly assigned. Check boxes track execution, not planning completion.

**Goal:** Establish the minimum safe Service recovery boundary, then determine whether Eino v0.9.13 satisfies the combined governed child and bounded Workflow lifecycle.
**Architecture:** Service owns logical operations, claims, authority, results and Journal truth. Eino owns scheduling and opaque checkpoints. Reuse existing storage transactions first; introduce only a necessary narrow storage seam, never a second runtime or general transaction platform.
**Tech Stack:** Go, pinned Eino v0.9.13, Service/broker/checkpoint bridge and existing SQL stores.
**Spec:** [architecture](../../specs/2026-09-23-issue39-eino-orchestration-design.md), D15 and G0/G1; [index](index.md) alone owns Story status and dependencies.

## Revised scope and historical evidence

The owner approved the D15 recovery design on 2026-09-26. The old test-only/sentinel scope is superseded for the next implementation attempt: minimum production recovery behavior and proof-node admission may be implemented here before G0, with test-first evidence. This removes the dependency on later ORCH-02 durability work. General ChildSession/mailbox schema, public controls, writable-child permissions and the workflow product remain out of scope.

The [prior Task 5 evidence](../../../logs/2026-09-26-issue39-native-proof/task-5-summary.md) remains unchanged. It observed two direct calls, not a recovered stable operation. Missing integration evidence is BLOCKED, not proof of Eino impossibility. The sentinel scaffold and intentional RED tests exist, but neither represents implementation or G0 acceptance.

## Files and interfaces

Inspect and minimally adapt `internal/runtime/{service,engine,toolbroker,orchestration,checkpoint,checkpointadapter}.go`, `internal/app/worker.go`, and existing Journal/Run/store contracts. Keep Eino imports quarantined. Reuse existing effect identity, transactions and lease fences where sufficient. If an atomic claim/result boundary cannot be expressed there, specify the exact narrow storage contract and paired append-only migrations under `internal/storage/migrations` before adding them. Do not create a separate execution truth.

Graph Run, proof-node admission, child activation Run and checkpoint identities must be durably bound. Tool operation identity is stable across recovery, not keyed by execution attempt or argument equality. The architecture D15 table is normative; this plan does not redefine it.

## Execution order

### Task 1: Recheck baseline and development prerequisites

Recheck branch baseline, pinned Eino and available Go/just/PowerShell/SQL tooling. Preserve the unchanged `TestServiceApprovalApproveFlow` baseline. Record missing prerequisites and PostgreSQL deferral honestly; deferred backend execution never proves parity.

### Task 2: Add failing operation-boundary cases

Add focused failing cases for same-key concurrent admission, conflicting payload, distinct same-argument calls, completion-write failure and recovery of claimed/finished operations. Replace the misleading direct-double-call assertion with identity-aware tests; preserve historical logs. A plain call twice without an operation identity is not a duplicate-recovery test.

### Task 3: Implement the minimum durable operation boundary

Implement the minimum Service-owned operation admission/atomic claim/result boundary under D15. Persist effective invocation binding, enforce existing authority/lease fences, propagate completion persistence failures and leave unknown effects fenced. Cover both ordinary and approved broker paths. Reuse Journal/storage first; justify any new schema and pair both dialects. This is the foundation before the integrated proof, not a waiver of schema acceptance.

### Task 4: Bind graph runs, prompts, checkpoints and proof-node admission

Implement the minimum shared Service graph Run/prompt/checkpoint binding and idempotent one-shot proof-node admission. Use Eino Workflow `End()`, `AddInput` and order-only `AddDependency`; never `WithMaxRunSteps` in DAG mode. Nodes call the governed Service boundary. No alternate model loop, scheduler, public API or child write expansion.

### Task 5: Prove the integrated native parallel workflow

Replace sentinel RED with integrated `A,B -> join` behavior: parallel overlap, explicit outputs only, distinct Run/checkpoint identities, approval interrupt while a sibling runs, cancellation, invalid engine/prompt failure and no orphan activation. Eino owns dependency execution; Service owns bounded admission and public evidence.

### Task 6: Prove D15 process-crash recovery

Execute D15 crash cases in a separate process with an observable durable effect outside the Journal transaction. Terminate before claim, after claim/before invocation, after effect/before completion and after completion/before graph checkpoint. Recover with fresh Service/Engine/store instances. Completed operations reuse results; uncertain operations visibly block with no re-invocation. Do not demand successful automatic continuation in the uncertain window.

### Task 7: Verify the recovery and native proof gates

Verify idempotent node admission and effects together. Capture operation/Run/checkpoint IDs, Journal sequences, effect counts and parent-visible outcomes. Run named focused runtime/storage tests and `just ci`; preserve evidence in a new attempt log. Do not overwrite the earlier NO-GO artifacts. Both SQL backends remain mandatory for schema acceptance; an owner-deferred backend is explicitly unverified.

**2026-09-26 execution evidence:** a full `go test ./...` passed before the final broker-policy fix. After that fix, the focused app/runtime suites, provider conformance, `go vet ./...`, storage and UI checks pass. The final full-suite rerun was blocked by automatic review because one test attempted an unapproved request to `api.deepseek.com`; it was not retried. Approved broker execution and replay reject a current `PolicyDeny`; approval satisfies `PolicyPrompt` only. The detailed commands and outcomes are in the [current attempt verification](../../../logs/2026-09-26-issue39-orchestration-g0/verification.md). G0 remains BLOCKED: `just ci` cannot run because `just` and PowerShell are absent, and PostgreSQL DSN-backed conformance cannot execute because `VIVY_POSTGRES_TEST_DSN` is unset. The separate-process crash test logs the stable Run, checkpoint, operation and Journal transition sequence together with external effect counts and recovery outcome; see the [acceptance record](../../../logs/2026-09-26-issue39-orchestration-g0/acceptance.md).

## Gate decision and handoff

GO requires the integrated native path and D15 safety matrix, with any deferred backend acceptance separately identified and still gated. Missing code, unavailable tooling or incomplete recovery scenarios mean BLOCKED. A demonstrated native incompatibility or unsafe replay in the tested path means NO-GO for that path and keeps downstream work closed pending correction or revised design; no custom executor fallback is authorized. A successful bounded proof is not a public product release or PostgreSQL parity claim.

Pass the proven operation/admission seam and evidence to ORCH-02, which extends it for task conversations and mailbox storage without duplicating tool-effect state. Preserve read-only child scope. Rollback disables new admission while retaining readable evidence and append-only migration history. Unknown effects must never be cleared merely to permit a rerun.
