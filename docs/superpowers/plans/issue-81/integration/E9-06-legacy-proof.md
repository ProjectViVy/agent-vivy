# E9-06 Retired Workflow Proof Isolation Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans after implementation authorization. Steps use checkbox (`- [ ]`) syntax.

**Goal:** Recover #32 W9 so obsolete direct-Eino proof code is test-only and legacy approvals cannot resume it.

**Architecture:** Preserve old rows and opaque checkpoint bytes, reject the legacy resume target before decision settlement, and keep the bounded compose proof only in test source. Current INOFY admission is unchanged.

**Tech Stack:** Go; existing Journal/checkpoint envelope; pinned Eino compose/schema only in runtime test source.

**Spec:** Old #32 P6 `fd1954f1` lineage within `6045c0a...`; architecture.md D-LEGACY. Status: package.md.

## Global Constraints

No sdk/host/v1, embedded/go-host, Tauri/C ABI restoration, historical row deletion or checkpoint rewrite. Preserve main approval ownership and #80 cleanup intent. No new engine/legacy compatibility runtime. Eino `compose.NewWorkflow`/Compile and schema registration are proof dependencies, not product authority.

## Review Focus

Legacy pending approval consumed before rejection; timed-system path bypassing actor guard; historic rows becoming unreadable; hidden production decoder needing a type registration; current workflow blocked by the legacy fence.

## Files and interfaces

Move executable proof from existing `internal/runtime/orchestration.go` to branch-only-to-recover `internal/runtime/orchestration_proof_test.go`. Recover minimal `internal/runtime/legacy_orchestration.go` and tests from old P6; reconcile approval/service dispatch, `orchestration_conformance_test.go` and binding tests.

Preserve public `Service.DecideApprovalAsActor(ctx context.Context, approvalID, decision, reason, actor string) error` and related entry points. Retain private `isNativeOrchestrationResumeTarget(target string) bool` for literal `native-orchestration:` and typed `ErrLegacyOrchestrationResumeUnsupported`. No new public API. Keep a schema registration in production only if a concrete live decoder is demonstrated; opaque envelope reading alone is not one.

## Tasks

- [ ] Inventory all non-test callers, decoder/schema registrations and persisted legacy-target handling on the selected candidate. Compare old P6 decisions with main #49/#50 and #80. Record exact required production symbols before moving source.
- [ ] Recover `TestLegacyOrchestrationApprovalRejectedBeforeDecision`, `TestLegacyOrchestrationApprovalAfterRestartRejected`, `TestLegacyOrchestrationHistoryReadable`, `TestLegacyOrchestrationDoesNotBlockCurrentINOFY`. Old pending target with actor/timed/restarted decisions returns the typed unsupported error, leaves pending decision unchanged and emits no new model/tool work. Schema-1 row/checkpoint remains byte-identical/readable; normal authored/trusted workflows still admit.
- [ ] Run `go test ./internal/runtime -run '^TestLegacyOrchestration' -count=1 -v` red. Add the minimal guard before first-writer settlement in all decision entry paths. Remove obsolete resume dispatch and runner, retaining supported current recovery. Never reject only after mutating the approval row.
- [ ] Move bounded proof and only required proof registrations into `_test.go`; remove old approval-specific executable helpers rather than preserving a parallel runtime in tests. Keep minimal production target detector/error and supported history readers.
- [ ] Run `go test ./internal/runtime -run '^Test(LegacyOrchestration|NativeOrchestration|Orchestration|INOFY|Cognitive)' -count=1`, full runtime tests and `go list -json ./internal/runtime`. Assert production GoFiles contains no proof builder/type/registration or compose.NewWorkflow path, TestGoFiles does retain the proof. Run required product `just ci`; a fabricated ui/dist placeholder is not product build acceptance.
- [ ] Independent reviewer verifies decoder inventory, before-decision guard, current approval/recovery preservation and no removed host imports. Record current candidate and unchanged legacy row/checkpoint digests; commit explicit paths with human attribution.

**Acceptance:** No product path compiles/resumes obsolete proof; old history remains readable; legacy decisions fail before effects; current INOFY and approval recovery work unchanged.

**Stop conditions:** A real production decoder/caller still depends on a moved symbol; public historical support differs from the accepted P6 contract; shared #80 guard/recovery conflict cannot be safely resolved locally. Record exact consumer; do not restore removed architecture or silently erase history.
