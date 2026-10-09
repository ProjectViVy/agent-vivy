# P6: Obsolete proof and historical safety implementation plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Remove the old Eino DAG proof from production while safely rejecting historical resume targets and retaining readable history.

**Architecture:** Executable proof belongs in test-only source. The production approval boundary keeps only a minimal legacy-target rejection check; it never routes obsolete approvals to generic agent resume. Existing Journal/checkpoint/history formats remain intact.

**Tech Stack:** Go `1.26.4`, Eino `v0.9.13` compose/schema, existing approval and checkpoint stores.

**Spec:** [P6 contract](../../specs/2026-10-09-issue32-remediation-design.md#p6-obsolete-proof-and-historical-safety).

## Global Constraints

- Preserve one Service/Journal/policy path; this task adds no workflow engine.
- Keep Eino at `v0.9.13`; only runtime/provider import Eino.
- Preserve historical schema-1 rows, descriptor_json, approvals and checkpoints.
- Retain `internal/embedded` and `sdk/host/v1`; do not retire Tauri/C ABI here.
- Sequence shared Service edits with P2/P5; use temporary stores only.
- Product `just ci` and relevant history/recovery checks remain required.

## Review Focus

- Old approval target reaches generic resume after deletion: reject before consuming its decision (P6.1).
- Old history decoder requires a registered name: prove the actual decode path before removing that registration (P6.1).
- Restart pending approvals differ from live pending approvals: both perform zero model/tool calls (P6.1).
- Schema-1 descriptor remains readable but cannot recover: preserve that explicit skip (P6.1).
- An authored/trusted current workflow is mistaken for proof history: current INOFY admission remains usable (P6.1).

---

### Task P6.1: Retire proof execution and close legacy resume safely (W9)

**Files:**
- Move executable proof from `internal/runtime/orchestration.go` to new `internal/runtime/orchestration_proof_test.go`.
- Create minimal production `internal/runtime/legacy_orchestration.go` for target detection/error only.
- Modify: `internal/runtime/service.go` approval entry/settlement path.
- Test: `internal/runtime/orchestration_conformance_test.go`; create `internal/runtime/legacy_orchestration_test.go`.
- Read: existing checkpoint envelope adapter, recovery tests and `docs/logs/2026-09-29-inofy-cutover/verification.md`.

**Interfaces:**
- Preserve `Service.DecideApprovalAsActor(ctx context.Context, approvalID, decision, reason, actor string) error` and related public entry points.
- Retain private `isNativeOrchestrationResumeTarget(target string) bool` checking the exact existing `native-orchestration:` prefix.
- Add `ErrLegacyOrchestrationResumeUnsupported`; all approval entry paths return it for a legacy target before first-writer decision settlement.
- Test-only proof keeps actual `compose.NewWorkflow`, runner/checkpoint helpers and registrations needed by proof tests. Live runtime retains a registration only when an identified production decoder needs it.

- [ ] **Step 1: Write historical safety regressions.** `TestLegacyOrchestrationApprovalRejectedBeforeDecision` constructs an old pending target, requests an allowed decision -> errors.Is unsupported, pending decision unchanged, no model/tool calls. `TestLegacyOrchestrationApprovalAfterRestartRejected` reopens the store and proves the same result. `TestLegacyOrchestrationHistoryReadable` reads schema-1 descriptor/checkpoint envelope without executing proof; compare original bytes unchanged. `TestLegacyOrchestrationDoesNotBlockCurrentINOFY` starts an ordinary authored and a trusted strategy workflow through current admission.
- [ ] **Step 2: Establish baseline behavior and decoder inventory.** Run `go test ./internal/runtime -run '^TestLegacyOrchestration' -count=1`; expected obsolete approval path does not return the explicit rejection. Search production callers and schema decoder usage with `rg -n 'nativeOrchestration|orchestrationProof|vivy_native_orchestration|vivy_orchestration_proof' internal sdk --glob '*.go'`. Record which non-test caller, if any, actually deserializes each registered type; storing opaque checkpoint bytes alone is not such a caller.
- [ ] **Step 3: Add the minimal production guard.** Check the fetched approval's ResumeTarget before decideApproval/first-writer settlement in every actor/system route that can consume it. Return the explicit unsupported error, preserving the old pending row. Remove the obsolete resumeNativeOrchestrationApproval dispatch; the guard prevents fallthrough into generic resume. Keep historical lookup/inspection behavior unchanged.
- [ ] **Step 4: Move executable proof to test-only source.** Move descriptors, graph construction, proof runner and proof-only registration code into orchestration_proof_test.go. Keep only the proved-needed decoder types/registrations in production, with comments naming the actual historical consumer. Remove proof-only production imports. Do not rewrite persisted rows or checkpoints and do not remove current workflow APIs.
- [ ] **Step 5: Verify green and production inventory.** Run `go test ./internal/runtime -run 'Test(LegacyOrchestration|NativeOrchestration|Orchestration|INOFY|Cognitive)' -count=1` and `go build ./...`. Run `go list -f '{{.GoFiles}}' ./internal/runtime`: the executable proof file is absent from production GoFiles, while proof tests still execute pinned Eino. Confirm all historical safety cases have zero side-effect calls and current workflows still start.
- [ ] **Step 6: Commit.** Commit `refactor(runtime): isolate orchestration proof and reject legacy resume` with the production-decoder inventory and focused evidence.

## Phase exit

Integrated `just ci`, history/recovery regressions and source inventory pass.
W9 is engineering-verified only when proof execution is absent from the product
and old resume cannot reach a model/tool. P7 owns final source-hash/conformance
refresh; transition C ABI/Tauri disposition remains governed by its W6 contract.
