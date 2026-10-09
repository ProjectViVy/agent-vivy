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
- Keep the bounded `compose.NewWorkflow` proof and only its needed type registrations in `_test.go`; remove the retired approval runner and resume adapter from runtime and test code. Live runtime retains a registration only when an identified production decoder needs it.

**Eino capability check:** The pinned Eino `v0.9.13` `compose.NewWorkflow` and
`Workflow.Compile` APIs support the old graph proof, and `schema.RegisterName`
provides typed checkpoint serialization. The repository's S11-E verification
inventory identifies no production caller of the proof builders; the only live
route was the obsolete approval resume target. After the guard, no product path
compiles or resumes that graph. Its Eino serialization registrations therefore
remain in test-only source for the proof test, while the existing versioned
checkpoint adapter continues to preserve/read opaque bytes without decoding
the retired graph state.

- [x] **Step 1: Write historical safety regressions.** `TestLegacyOrchestrationApprovalRejectedBeforeDecision` constructs an old pending target, requests an allowed decision -> errors.Is unsupported, pending decision unchanged, no model/tool work events. `TestLegacyOrchestrationApprovalAfterRestartRejected` repeats the decision through a fresh Service and exercises the timed system path. `TestLegacyOrchestrationHistoryReadable` reads a schema-1 descriptor and opaque checkpoint without decoding or rewriting either. `TestLegacyOrchestrationDoesNotBlockCurrentINOFY` starts an authored workflow and a trusted strategy workflow through current admission.
- [x] **Step 2: Establish baseline behavior and decoder inventory.** Ran `go test ./internal/runtime -run '^TestLegacyOrchestration' -count=1` before the guard. The live approval was consumed and returned nil; post-restart approval was likewise consumed. Production inventory found no non-test proof builder or decoder; `VersionedCheckpointStore.Get` only verifies the VIVY envelope and returns opaque Eino bytes.
- [x] **Step 3: Add the minimal production guard.** Guard actor decisions, common decision settlement and timed system settlement before first-writer mutation. Return `ErrLegacyOrchestrationResumeUnsupported`; keep the old row pending. Remove the obsolete Service dispatch to native proof resume.
- [x] **Step 4: Move executable proof to test-only source.** Move the Eino graph proof and required type registrations into `orchestration_proof_test.go`; remove the old approval-specific graph execution/resume helpers and their approval-resume integration tests. Production `GoFiles` contains only the small marker detector and typed rejection error. Historical rows and blobs remain readable and unchanged.
- [x] **Step 5: Verify green and production inventory.** `go test ./internal/runtime -run '^Test(LegacyOrchestration|NativeOrchestration|Orchestration|INOFY|Cognitive)' -count=1`, `go test ./internal/runtime -count=1`, and `go build ./...` passed. The build used a temporary `ui/dist/.keep` because this checkout has no embedded UI build output; that placeholder was removed after the build. `go list` excludes the proof from production GoFiles and includes it in TestGoFiles. Production `rg` finds no old graph builder, proof type or registration references.
- [x] **Step 6: Commit (`fd1954f1`).** Committed as `refactor(runtime): isolate orchestration proof and reject legacy resume` with the production-decoder inventory and focused evidence.

## Phase exit

W9 is engineering-verified after the runtime tests, product build and source
inventory pass; the product `just ci` gate remains pending because `just` is not
installed in this environment. P7 owns final source-hash/conformance refresh;
transition C ABI/Tauri disposition remains governed by its W6 contract.
