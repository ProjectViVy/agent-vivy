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

- [ ] **Step 1: Write failing regressions.** `TestCognitiveStateStaleVersionRejected`: load v0, accept capture seq=9, save old state with v0 -> `errors.Is(err, storage.ErrVersionConflict)`, stored SourceHigh=9. `TestCognitiveConcurrentCapturePolicyAndSettlement`: barrier-controlled capture/policy/settlement preserve maximum SourceHigh, policy revision, exact completed watermark and block. `TestCognitivePolicyCASRejectsStaleRevision`: two equal base revisions produce one success and one ErrPolicyConflict.
- [ ] **Step 2: Verify red.** Run `go test ./internal/runtime -run '^TestCognitive(StateStaleVersionRejected|ConcurrentCapturePolicyAndSettlement|PolicyCASRejectsStaleRevision)$' -count=1`. Expected: baseline stale save is accepted or loses seq=9; no sleeps are used to force the race.
- [ ] **Step 3: Implement the interfaces above.** Carry the originally loaded version to every save. Pure mutations reload under cogStateMu and may return validation errors; policy validation and increment occur inside that mutation. Attempt commits after external reads use explicit expected versions. Release state lock before effectful calls; conflicts after possible effects reconcile, never replay the whole attempt. Update every old helper caller and serialize manual/automatic attempts with cogAttemptMu.
- [ ] **Step 4: Verify green and concurrency.** Run the focused command, then `go test -race ./internal/runtime -run '^TestCognitive' -count=1`. Expected: target regressions pass; no lock inversion or lost accepted transition. Report any unrelated known Eino race separately rather than weakening the test.
- [ ] **Step 5: Commit.** Stage only runtime changes/tests and the phase log. Commit `fix(cognitive): preserve original snapshot CAS versions`.

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

- [ ] **Step 1: Write the fault matrix.** `TestCognitiveIntentCrashMatrix` reopens stores after intent save, admission commit, completion and state settlement. Assert one admitted Run and one effect for completed cases; Watermark equals persisted Through. Include active adoption, retry-safe failure, cancellation, unknown persistence/lookup and replaced supervisor. `TestCognitiveIntentPreservesLaterInput` completes Through=9 after accepting seq=10 -> Watermark=9, SourceHigh=10. `TestCognitiveLegacyCrashGapFencesUnknown` proves legacy missing-intent ambiguity does not replay or fabricate a watermark. `TestCognitiveRetrySafetyRequiresSettlementEvidence` contrasts a fully settled definite failure with a missing model.call.finished, unknown effect and failed workflow lookup; only the fully proved safe failure may retry.
- [ ] **Step 2: Verify red.** Run `go test ./internal/runtime -run '^TestCognitive(IntentCrashMatrix|IntentPreservesLaterInput|LegacyCrashGapFencesUnknown|RetrySafetyRequiresSettlementEvidence)$' -count=1`. Expected: baseline lacks pre-admission intent, treats completed dedupe as another attempt, or assumes lookup failure is retry-safe.
- [ ] **Step 3: Implement intent-first admission and the reconciler.** Persist immutable input/identity and PendingThrough before starting. Reconcile original parent/key before ensureCognitiveSupervisor can replace the parent. Exact completed result settles its window; cancelled/unknown blocks. Retry-safe failure uses the same window and next bounded attempt only after proving no ambiguous effects. If an unadmitted persisted parent's lifecycle is invalid, fence instead of silently rebinding it. Fresh input cannot alter an outstanding intent. Upgrade legacy snapshots once to StateSchema=2: known ActiveRunID reconciles from durable evidence; an outstanding missing-intent window without proof of safety gets unknown_outcome; empty/idle state upgrades without a fence. Reject unknown future schema values and test reopen of every shape.
- [ ] **Step 4: Verify green.** Run the fault matrix plus `go test ./internal/runtime -run '^TestCognitive' -count=1` and `go test ./internal/storage/sqlite ./internal/storage/postgres -run 'TestBackendConformance/CN-03|TestSnapshotVersions' -count=1 -v` with an actual disposable VIVY_POSTGRES_TEST_DSN. Expected: crash cuts converge without duplicate effects; cancellation/unknown stay blocked; reopen keeps identity/pins and both backends reject stale versions. A skipped DSN-backed test is pending.
- [ ] **Step 5: Commit.** Commit `fix(cognitive): reconcile persisted admission windows` with the fault-cut outcomes in verification.md.

### Task P2.3: Current Mission and single durable policy pin (C3, C4)

**Files:**
- Modify: `internal/runtime/cognitive_service.go`, `internal/runtime/cognitive_binding.go`, `internal/cognitivecontract/ports.go`.
- Modify: `internal/app/app.go`, `internal/modules/diva-cognitive/factory.go`, `dispatch.go` and their fake implementations.
- Test: `internal/runtime/cognitive_binding_test.go`, `cognitive_service_test.go`, `internal/modules/diva-cognitive/factory_test.go`, `actions_test.go`, `internal/app/assembly_cognitive_test.go`.

**Interfaces:**
- Consumes: P2.1 durable policy and P2.2 immutable run input.
- Produces: `Bundle.ResolveBinding(ctx context.Context, policy laputaevolution.TriggerPolicy) (laputaevolution.RunBinding, error)`; matching `CognitiveBinding.Resolve func(context.Context, laputaevolution.TriggerPolicy) (laputaevolution.RunBinding, error)`.
- `Bundle.Policy()` remains an immutable first-load seed. Delete mutable policy/setPolicy; resolver hashes its policy argument. Existing `BoundDomain` and persisted-pin guard remain authoritative.

- [ ] **Step 1: Write regressions.** `TestCognitiveCurrentMissionBindingAdmitted`: edit Mission from revision1 to2 -> new run input pins2. `TestCognitiveMissionChangesAfterResolutionFailClosed`: edit again before effect -> no effect under pin2. `TestCognitivePolicyPinSurvivesReopen`: persist enabled/nondefault interval, close/reopen bundle/service -> identical policy digest and eligibility; test concurrent update pins the one policy snapshot used for that intent.
- [ ] **Step 2: Verify red.** Run `go test ./internal/runtime ./internal/modules/diva-cognitive ./internal/app -run 'TestCognitive(CurrentMissionBindingAdmitted|MissionChangesAfterResolutionFailClosed|PolicyPinSurvivesReopen)' -count=1`. Expected: current old-binding check or volatile policy digest fails.
- [ ] **Step 3: Implement the resolver seam.** Validate resolved.CheckMissionRevision, not the construction-time binding; retain fixed scope/strategy checks and execution-time persisted-pin validation. Pass the same durable policy into binding resolution and intent creation. Update core seam, Module, App and all mocks together; remove dispatch's second policy update. Regenerate selected Assembly through SDK if its contract output changes; do not hand-edit it.
- [ ] **Step 4: Verify green.** Run the focused tests and full affected package tests. Reopen before checking actual persisted input bytes/digest; source-only equality is insufficient. Record the Module/Port/Provider/Consumer evidence and unchanged Eino capability check in the phase log.
- [ ] **Step 5: Commit.** Commit `fix(cognitive): pin current durable authority per admission`.

### Task P2.4: Consistent status and scoped cancellation (C5, C6)

**Files:**
- Modify: `internal/runtime/cognitive_service.go`, `internal/app/assembly_cognitive.go`, `internal/cognitivecontract/ports.go`, `internal/modules/diva-cognitive/dispatch.go`.
- Test: `internal/app/assembly_cognitive_test.go`, `internal/modules/diva-cognitive/actions_test.go`, `internal/runtime/cognitive_service_test.go`.
- Verify/update DIVA: `agent-diva-gui/src/components/EvolutionView.vue` and its tests if consumers need the truthful existing fields.

**Interfaces:**
- Consumes: P2.1/P2.2 state version, Phase and intent; existing ControlState fields.
- Produces: `Service.CognitiveControlState(ctx context.Context) (cognitivecontract.ControlState, error)` and `Service.CancelCognitiveRun(ctx context.Context, runID domain.RunID) (bool, error)`.
- Add `ErrCognitiveRunMismatch` for a foreign/stale/non-strategy run. ControlPort.Cancel signature stays unchanged and calls only the scoped method.

- [ ] **Step 1: Write regressions.** `TestCognitiveControlStateOneSnapshot` returns coherent policy, ActiveRunID, PendingThrough, Phase and BlockReason from one Get; fail a second Get to prove projection has no second read. `TestCognitiveControlProjectionVerbatim` checks dispatch JSON for pending/phase/block. `TestCognitiveCancelScopeMatrix` covers exact strategy, foreground, foreign supervisor, stale ID and concurrent settlement -> only current bound strategy can reach CancelRun.
- [ ] **Step 2: Verify red.** Run `go test ./internal/runtime ./internal/app ./internal/modules/diva-cognitive -run 'TestCognitive(ControlStateOneSnapshot|ControlProjectionVerbatim|CancelScopeMatrix)' -count=1`. Expected: current projection drops fields or arbitrary ID reaches cancellation.
- [ ] **Step 3: Implement the methods.** Persist Phase at corresponding transitions; read one snapshot for projection. Under attempt serialization resolve exact ActiveRunID and verify strategy kind/scope before cancellation; do not hold state lock during CancelRun. Settlement races yield a truthful no-longer-active outcome. App and dispatcher project the existing fields without inventing another scheduler/status store.
- [ ] **Step 4: Verify green and actual adapter.** Run focused/full affected tests. Exercise capture -> pending -> trigger -> running -> settle/block -> scoped cancel through actual cognitive actions; DIVA UI must show the same pending/phase/block fields. Use configured local fixtures, not tenant data.
- [ ] **Step 5: Commit.** Commit `fix(cognitive): project durable state and scope cancellation`.

## Phase exit

Run VIVY `just ci` on the integrated phase, selected cognitive SDK/Port
conformance and the real action path. Record four distinct recovery outcomes
and untouched foreground cancellation in the phase log. P7 performs final
source-bound evidence generation after all internal edits; phase-local evidence
must not be represented as the final DIVA artifact's evidence.
