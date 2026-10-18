# E9-01 Cognitive Exact Window Recovery Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans after implementation authorization. Steps use checkbox (`- [ ]`) syntax.

**Goal:** Selectively recover #32 C2 so a completed cognitive window cannot replay its effects after a crash.

**Architecture:** Reuse old P2.2 intent-first admission in the existing `cognitive/state` SnapshotStore record. Preserve current main original-version CAS, accepted capture semantics, unresolved effect fences and mandatory model settlement.

**Tech Stack:** Go 1.26.4, SnapshotStore CAS, existing INOFY workflow revision/dedupe and Laputa effect receipts.

**Spec:** Existing #32 P2-cognitive.md task P2.2 and `2026-10-09-issue32-remediation-design.md` at `6045c0a69a74c04a1a6221d93c0d7bb32ae7a6e1`; architecture.md D-INTENT. Status: package.md.

## Global Constraints

No new scheduler, model loop, runtime or SQL schema. C1/R4 already have main fixes; preserve their methods/tests. This is VIVY runtime correctness even though the existing strategy is named DIVA; no DIVA product acceptance or new #6/#7 capability is scheduled.

## Review Focus

Crash before/after admission; source high-water advances during restart; terminal dedupe mistaken for a failed attempt; missing terminal projection or settlement; legacy state whose effects cannot be determined. These are exact fault-cut assertions below, not timing-dependent sleeps.

## Files and interfaces

Modify current `internal/runtime/cognitive_service.go`; extend existing `cognitive_recovery_test.go`, `cognitive_service_test.go`, `cognitive_settlement_test.go` as appropriate. Read existing `workflow_service.go`, `workflow_revisions.go`/actual storage contracts and source-fence code. Only reconcile `cognitive_binding.go` if the existing admission checks need it; E9-02 owns resolver changes.

Consume unchanged `StartCognitiveWorkflow(ctx context.Context, parentRunID domain.RunID, operationKey, strategyID string, input json.RawMessage) (WorkflowStartResult, error)` and current original-version save helper. Reuse old branch:

```go
type cognitiveIntent struct {
    ParentRunID domain.RunID
    OperationKey string
    StrategyID string
    Input json.RawMessage
    Attempt int
}
func (s *Service) reconcileCognitiveIntent(ctx context.Context, st cognitiveState, version int64) (cognitiveState, int64, error)
func (s *Service) cognitiveRunRetrySafe(ctx context.Context, runID domain.RunID) (bool, error)
```

Preserve old branch JSON field tags. `cognitiveState` adds optional Intent, Phase and StateSchema=2. Missing schema means legacy; unknown future schema fails closed. Return updated committed state/version, never a refreshed version attached to stale bytes.

## Tasks

### 1. Recover fault matrix before correction

- [ ] Recover only old P2.2 tests and helpers that do not exist on main: `TestCognitiveIntentCrashMatrix`, `TestCognitiveIntentCrashBeforeAdmissionReusesIdentity`, `TestCognitiveIntentPreservesLaterInput`, `TestCognitiveLegacyCrashGapFencesUnknown`, `TestCognitiveRetrySafetyRequiresSettlementEvidence`. Align fixture APIs to current main, not production APIs to old fixtures.
- [ ] Matrix cuts: before intent persist; after intent before workflow admit; after admit before state update; after terminal native row before workflow settlement; after completed projection before state settlement. Close/reopen real SQLite between cuts. Assert original parent/key/input, exactly one workflow/effect identity, completed watermark equals recorded Through=9 even when current source high=10, and subsequent work begins after 9 rather than replaying 0..10.
- [ ] Failed with proven effect-free settled evidence may advance attempt within the existing budget. Cancelled, missing lookup, unknown effects, incomplete model settlement, unsupported schema and inconsistent attempt/window remain fenced. Run `go test ./internal/runtime -run '^TestCognitive(Intent|Legacy|RetrySafety)' -count=1 -v` and record the specific pre-fix behavioral failures.

### 2. Selective intent and reconciliation transplant

- [ ] Port the old intent-first state transition into current `cognitiveAttempt`: persist canonical parent/key/strategy/input/attempt/PendingThrough before launch; reconcile original identity before new supervisor creation or new source window. Validate Window.After against current watermark, Through against PendingThrough and operation-key attempt against stored attempt.
- [ ] Adopt existing active/complete workflow by exact persisted revision; completed + resolved receipts settles exact Through. Cancelled or unknown cannot mint new identity. A native failed Run whose workflow projection is not settled retains intent until evidence resolves; absence of error text is not retry safety.
- [ ] Preserve the legacy reconstruction/fence behavior from old P2.2: reconstruct only a provable ActiveRunID revision; outstanding legacy input without original identity blocks as unknown rather than replaying. Preserve opaque history. Manual wake cannot clear unknown outcomes; no new SQL migration or database owner.
- [ ] Port old follow-up regressions for key/attempt mismatch, orphan supervisor adoption, window-start mismatch and failed projection settlement. Keep main `CognitiveStatusView` rather than replacing it with the old branch's narrower TriggerState.
- [ ] Run focused tests, `go test -race ./internal/runtime -run '^TestCognitive' -count=1`, runtime package and SnapshotVersions/storage conformance. Expected: all actual selected cases execute and pass; report PostgreSQL skipped DSN rows as pending.

### 3. Integration and review

- [ ] Run `just ci` on exact candidate. Verify ordinary primary runs and unbound cognitive compositions are unchanged. Record old/new snapshot examples and actual reopen fault outcomes without raw tenant Journal data.
- [ ] Independent reviewer checks intent-before-effect, retry evidence, legacy fencing and main CAS/settlement preservation. Commit explicit paths with human attribution; supply E9-02/03 the schema/interface and candidate SHA.

**Acceptance:** No completed/unknown window is silently reissued under a new effect identity; safe failure retry and exact-window settlement remain distinct. Existing capture and foreground behavior survives.

**Stop conditions:** Current revision/effect evidence cannot establish outcome; selected dependency violates receipt contract; a proposed repair would discard unknown effects or add a second intent authority. Preserve fence and report exact cut; do not fabricate completion.
