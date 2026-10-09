# P2: Cognitive state and recovery implementation plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Preserve concurrent accepted state and reconcile exactly the cognitive window admitted before a crash.

**Architecture:** Service owns all trigger state in its existing SnapshotStore record. Original-version CAS and separate state/admission serialization protect transitions; a durable intent bridges admission and restart. Current binding, durable policy and control projection share that authority.

**Tech Stack:** Go `1.26.4`, SnapshotStore, INOFY pinned version, Laputa evolution contracts, Eino `v0.9.13`, generated cognitive Module/ports.

**Spec:** [P2 contract](../../specs/2026-10-09-issue32-remediation-design.md#p2-cognitive-state-and-recovery).

## Global Constraints

- Preserve one Service/Journal/policy path; no second runtime, scheduler or persistence service.
- Keep Eino at `v0.9.13`; only runtime/provider import Eino.
- Core Storage owns DDL; selected design adds snapshot JSON fields, not a table.
- Never hand-edit generated Assembly; regenerate through the SDK.
- No automatic replay of unknown effects; test with temporary stores and synthetic failures.
- Overlapping Service/App/Module work stays in one lane or merges before final evidence.

## Review Focus

- Capture races policy/settlement: seq=9 and accepted policy both survive (P2.1).
- Crash returns under a replaced supervisor: original parent/key finds the original Run (P2.2).
- New input arrives during work: completing Through=9 does not consume seq=10 (P2.2).
- Reopen/live Mission edit: next run pins durable current authority; admitted stale run rejects effects (P2.3).
- Cancel races settlement/foreground work: unrelated Run receives zero cancellation (P2.4).

---

## File responsibilities and prerequisites

P0 supplies reviewed pins. P2.1 precedes the other tasks. Runtime record,
intent and transition helpers remain in `internal/runtime/cognitive_service.go`;
no broad Service split is required. `cognitive_binding.go` owns trusted execution
pin checks; `cognitivecontract/ports.go` owns the core typed seam; App adapts it;
the selected Module resolves its own authority without retaining mutable policy.

### Task P2.1: Original-version state transitions (C1)

**Files:**
- Modify: `internal/runtime/cognitive_service.go`, `internal/runtime/service.go`.
- Test: `internal/runtime/cognitive_service_test.go`.
- Read: `internal/storage/contracts.go` SnapshotStore and `ErrVersionConflict`.

**Interfaces:**
- Consumes: `SnapshotStore.Get(ctx, key) ([]byte, int64, error)` and `Put(ctx, key, raw, expectVersion) error`.
- Produces: `saveCognitiveState(ctx context.Context, st cognitiveState, expectedVersion int64) error` and `updateCognitiveState(ctx context.Context, mutate func(*cognitiveState) error) error`.
- Add Service-owned `cogStateMu` for pure load/mutate/put and `cogAttemptMu` for admission/reconciliation. Lifecycle `cogMu` keeps its present purpose.

- [x] **Step 1: Write failing regressions.** `TestCognitiveStateStaleVersionRejected`: load v0, accept capture seq=9, save old state with v0 -> `errors.Is(err, storage.ErrVersionConflict)`, stored SourceHigh=9. `TestCognitiveConcurrentCapturePolicyAndSettlement`: barrier-controlled capture/policy/settlement preserve maximum SourceHigh, policy revision, exact completed watermark and block. `TestCognitivePolicyCASRejectsStaleRevision`: two equal base revisions produce one success and one ErrPolicyConflict.
- [x] **Step 2: Verify red.** Ran `go test ./internal/runtime -run '^TestCognitive(StateStaleVersionRejected|ConcurrentCapturePolicyAndSettlement|PolicyCASRejectsStaleRevision)$' -count=1`. All three new tests failed as expected: stale save lost seq=9; concurrent state writes surfaced a snapshot version conflict; equal-base policy CAS returned two successes. No sleeps force the race.
- [x] **Step 3: Implement the interfaces above.** Carry the originally loaded version to every save. Pure mutations reload under cogStateMu and may return validation errors; policy validation and increment occur inside the mutation. Attempt commits after external reads use explicit expected versions. Release state lock before effectful calls; on conflict, rebase only fields changed by the attempt onto the latest snapshot and do not replay external work. Update every old helper caller and serialize manual/automatic attempts with cogAttemptMu.
- [x] **Step 4: Verify green and concurrency.** Focused regressions and `go test -race ./internal/runtime -run '^TestCognitive' -count=1` passed. `go vet ./internal/runtime`, gofmt, and `git diff --check` passed. Aggregate `just ci` was unavailable because this environment has no `just` executable; details are in the P2.1 verification log.
- [x] **Step 5: Commit.** Committed as `fix(cognitive): preserve original snapshot CAS versions` on the isolated remediation branch.

### Task P2.2: Durable admission intent and restart reconciliation (C2)

**Files:**
- Modify: `internal/runtime/cognitive_service.go`, `internal/runtime/cognitive_binding.go` only if shared admission checks require it.
- Test: `internal/runtime/cognitive_recovery_test.go`, `internal/runtime/cognitive_service_test.go`.
- Read: `internal/runtime/workflow_service.go`, `internal/storage/workflow_revisions.go` and actual revision types.

**Interfaces:**
- Consumes: P2.1 state helpers; existing `StartCognitiveWorkflow(ctx context.Context, parentRunID domain.RunID, operationKey, strategyID string, input json.RawMessage) (WorkflowStartResult, error)`; revision lookup by original parent/key.
- Produces: `cognitiveIntent{ParentRunID domain.RunID, OperationKey string, StrategyID string, Input json.RawMessage, Attempt int}` persisted as optional `cognitiveState.Intent`; `cognitiveState.Phase string` and `StateSchema int` (2 for new/upgraded state, absent means legacy).
- Add `reconcileCognitiveIntent(ctx context.Context, st cognitiveState, version int64) (cognitiveState, int64, error)`; it returns the freshly committed state/version or preserves a fence/error, never constructs a new identity for an ambiguous prior run.
- Add `cognitiveRunRetrySafe(ctx context.Context, runID domain.RunID) (bool, error)`: use existing workflow/node/effect evidence and inference-child Journal records. A lookup error, unknown node/effect or started model call without mandatory settlement/finish evidence is not retry-safe. Do not infer safety from the absence of an error string.

- [x] **Step 1: Write the fault matrix.** Added `TestCognitiveIntentCrashMatrix`, with SQLite reopen before settlement and after settlement, plus active-Service adoption after admission; `TestCognitiveIntentCrashBeforeAdmissionReusesIdentity` closes/reopens SQLite before admission. Tests assert exact parent/key/input, one workflow/effect, completed Through=9 with SourceHigh=10, retry attempt identity, cancellation, unknown parent/lookup and schema behavior. Added `TestCognitiveIntentPreservesLaterInput`, `TestCognitiveLegacyCrashGapFencesUnknown`, `TestCognitiveLegacyActiveRunReconstructsIntent`, `TestCognitiveRetrySafetyRequiresSettlementEvidence`, `TestCognitiveOrphanSupervisorIsAdoptedBeforeAdmission`, `TestCognitiveLegacyFailedRunPreservesRetryBudget`, `TestCognitiveIntentKeyAttemptMismatchFencesBeforeAdmission`, `TestCognitivePersistedAttemptMismatchDoesNotRetry`, `TestCognitiveIntentWindowStartMustMatchWatermark`, and projection-state coverage in `TestCognitiveRecoveryClassifiesRunning` / `TestCognitiveFailedRunWaitsForDurableProjection`.
- [x] **Step 2: Verify red.** The planned command initially failed for the intended gaps: no schema-2 intent was saved before admission; a legacy SourceHigh=9/Watermark=0 gap was replayed; and workflow lookup failure was considered retry-safe. A follow-up regression also exposed that a failed Run row without a terminal durable workflow projection was treated as settled. Each failure was observed before its corresponding fix.
- [x] **Step 3: Implement intent-first admission and the reconciler.** Persist canonical input, original parent/key, strategy and attempt with PendingThrough before starting. Reconcile that identity before supervisor replacement, validating source, `Window.After == Watermark`, `Window.Through == PendingThrough`, and key/attempt consistency before admission and again against the persisted revision. An orphan deterministic supervisor is adopted if active or skipped if terminal. Active native Runs validate their workflow projection; recovery_required and unknown node outcomes fence. A native failed Run whose workflow projection is still settling retains its exact intent for a later wake; only complete effect-free evidence advances its attempt and can clear a provisional unknown fence. Completed settles the intent's exact Through; cancellation fences. Legacy ActiveRunID reconstructs attempt and window from its immutable operation key/revision and validates the snapshot attempt; ambiguous legacy outstanding input, including SourceHigh above Watermark without PendingThrough, fences as unknown_outcome. Future/unsupported schemas fail closed.
- [x] **Step 4: Verify green.** The planned fault-matrix command, all cognitive tests, cognitive race tests, full `internal/runtime` tests, `go vet ./internal/runtime`, `git diff --check`, SQLite CN-03 and SnapshotVersions passed. Review regressions were observed red before fixes; retry/projection tests passed ten repeated runs. PostgreSQL conformance was skipped because `VIVY_POSTGRES_TEST_DSN` is unset and remains pending. Aggregate `just ci` is unavailable because `just` is not installed.
- [x] **Step 5: Commit and independent review.** Committed locally as `b69a4b83` (`fix(cognitive): reconcile persisted admission windows`), then closed the independent-review regressions in `93ac1800`, `5f7deed3`, and `2de1aa18`. A fresh review of the final code found no remaining findings. Fault-cut outcomes and pending gates are recorded in verification.md.

### Task P2.3: Current Mission and single durable policy pin (C3, C4)

**Files:**
- Modify: `internal/runtime/cognitive_service.go`, `internal/runtime/cognitive_binding.go`, `internal/cognitivecontract/ports.go`.
- Modify: `internal/app/app.go`, `internal/modules/diva-cognitive/factory.go`, `dispatch.go` and their fake implementations.
- Test: `internal/runtime/cognitive_binding_test.go`, `cognitive_service_test.go`, `internal/modules/diva-cognitive/factory_test.go`, `actions_test.go`, `internal/app/assembly_cognitive_test.go`.

**Interfaces:**
- Consumes: P2.1 durable policy and P2.2 immutable run input.
- Produces: `Bundle.ResolveBinding(ctx context.Context, policy laputaevolution.TriggerPolicy) (laputaevolution.RunBinding, error)`; matching `CognitiveBinding.Resolve func(context.Context, laputaevolution.TriggerPolicy) (laputaevolution.RunBinding, error)`.
- `Bundle.Policy()` remains an immutable first-load seed. Delete mutable policy/setPolicy; resolver hashes its policy argument. The selected Garden owner must atomically validate each persisted Mission revision while applying the effect under a shared read/write gate with human Mission writes. A Mission-wired Domain without this gate fails closed.

- [x] **Step 1: Write regressions.** Added `TestCognitiveCurrentMissionBindingAdmitted`, `TestCognitiveMissionChangesAfterResolutionFailClosed`, `TestCognitivePolicyPinSurvivesReopen`, `TestCognitiveConcurrentPolicyUpdatePinsAdmissionSnapshot`, and `TestResolveBindingPinsSuppliedPolicy`.
- [x] **Step 2: Verify red.** Before the resolver correction, `TestCognitiveCurrentMissionBindingAdmitted` failed with `mission_revision_changed` when construction-time Mission revision 1 differed from current/resolved revision 2.
- [x] **Step 3: Implement the resolver and atomic Mission gate.** `Bundle.ResolveBinding` and `CognitiveBinding.Resolve` accept the same durable policy snapshot loaded for admission. The resolver hashes that argument; mutable bundle policy and post-write `setPolicy` were removed. Admission and executor creation validate the resolved Mission pin. Each read and effect rechecks it; the production Garden adapter holds the owner lifecycle read lock and shared Mission read gate from revision check through Domain.Apply. Human initialization, direct Mission saves and review decisions take the paired write gate. Mission-wired adapters without atomic apply fail closed. App wiring and all fakes use the typed policy seam; no generated Assembly contract changed.
- [x] **Step 4: Verify green and independent review.** Full `internal/runtime`, `internal/modules/diva-cognitive`, `internal/app`, and Garden `agentapi` tests passed; focused atomic-gate regressions passed. Garden `go vet ./agentapi` and `git diff --check` passed. App tests used a temporary embedded `ui/dist/.keep`, then removed it. UI build remains blocked by the configured package registry; Eino remains pinned at `v0.9.13`, and no SDK export changed. A fresh independent review found no remaining findings. Evidence is in [P2.3 verification](../../../logs/2026-10-09-issue32-p2.3-current-policy-pin/verification.md).
- [x] **Step 5: Commit.** Laputa owner gate committed as `30fa208` (`fix(garden): guard mission-pinned effect application`); VIVY P2.3 changes and phase evidence committed as `35e559f6` (`fix(cognitive): pin current Mission and durable policy`).

### Task P2.4: Consistent status and scoped cancellation (C5, C6)

**Files:**
- Modify: `internal/runtime/cognitive_service.go`, `internal/app/assembly_cognitive.go`, `internal/cognitivecontract/ports.go`, `internal/modules/diva-cognitive/dispatch.go`.
- Test: `internal/app/assembly_cognitive_test.go`, `internal/modules/diva-cognitive/actions_test.go`, `internal/runtime/cognitive_service_test.go`.
- Verify/update DIVA: `agent-diva-gui/src/components/EvolutionView.vue` and its tests if consumers need the truthful existing fields.

**Interfaces:**
- Consumes: P2.1/P2.2 state version, Phase and intent; existing ControlState fields.
- Produces: `Service.CognitiveControlState(ctx context.Context) (cognitivecontract.ControlState, error)` and `Service.CancelCognitiveRun(ctx context.Context, runID domain.RunID) (bool, error)`.
- Add `ErrCognitiveRunMismatch` for a foreign/stale/non-strategy run. ControlPort.Cancel signature stays unchanged and calls only the scoped method.

- [x] **Step 1: Write regressions.** Added `TestCognitiveControlStateOneSnapshot` with a store that rejects a second read, app ControlPort projection of every durable field, dispatch JSON projection of pending/phase/block, and `TestCognitiveCancelScopeMatrix` for the exact strategy, foreground/foreign/stale IDs and post-settlement cancellation.
- [x] **Step 2: Verify red.** Before implementation, the runtime regression package failed to compile because `CognitiveControlState`, `CancelCognitiveRun`, and `ErrCognitiveRunMismatch` did not exist. This established the missing service seams; DIVA's existing dispatcher already copied fields supplied by its ControlPort.
- [x] **Step 3: Implement the methods.** Added a one-snapshot control projection with legacy phase derivation. Cancellation serializes with admission, validates the durable intent and run lineage, then rechecks status and signals cancellation under `projectionMu`, shared with terminal persistence. The generic cancellation path is not called while that lock is held because a suspended run may synchronously emit its terminal event. App uses only these scoped methods.
- [x] **Step 4: Verify green and actual adapters.** Independent review found and the red regression reproduced a terminal-wins race; the shared terminal gate fixed it. Full runtime/module/App tests, focused Race detection, `go vet` and `git diff --check` passed. App ControlPort and module action tests verified coherent output and cancellation scope. DIVA `EvolutionView.vue` already reads all recovery fields, so no UI source change was required. Evidence is in [P2.4 verification](../../../logs/2026-10-09-issue32-p2.4-control-cancel/verification.md).
- [ ] **Step 5: Commit.** Commit `fix(cognitive): project durable state and scope cancellation`.

## Phase exit

Run VIVY `just ci` on the integrated phase, selected cognitive SDK/Port
conformance and the real action path. Record four distinct recovery outcomes
and untouched foreground cancellation in the phase log. P7 performs final
source-bound evidence generation after all internal edits; phase-local evidence
must not be represented as the final DIVA artifact's evidence.
