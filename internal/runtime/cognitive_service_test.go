package runtime

import (
	"context"
	"encoding/json"
	"errors"
	"path/filepath"
	"sync"
	"testing"
	"time"

	laputaevolution "github.com/ProjectViVy/laputa/laputa/evolution"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/observerhost"
	"agent-vivy/internal/storage"
	"agent-vivy/internal/storage/sqlite"
	"agent-vivy/sdk/port/observer"
)

// fakeSource supplies a scripted committed-activity watermark.
type fakeSource struct{ high uint64 }

func (f *fakeSource) HighWatermark(context.Context) (uint64, error) { return f.high, nil }

// fakeSink dedupes captures by event ID like the durable ingest surface.
type fakeSink struct {
	seen     map[string]CognitiveCaptureReceipt
	captures []CognitiveCapture
}

func (f *fakeSink) Capture(_ context.Context, c CognitiveCapture) (CognitiveCaptureReceipt, error) {
	if f.seen == nil {
		f.seen = map[string]CognitiveCaptureReceipt{}
	}
	if r, ok := f.seen[c.EventID]; ok {
		return r, nil
	}
	r := CognitiveCaptureReceipt{IngestionID: "ing_" + c.EventID, Seq: uint64(len(f.captures)) + 1, Status: "accepted"}
	f.seen[c.EventID] = r
	f.captures = append(f.captures, c)
	return r, nil
}

func cognitiveTestModel() *scriptedModel {
	return &scriptedModel{replies: map[string]string{
		"stage=reconcile": `{"base_revision":3,"changes":[{"kind":"add","entry_id":"","field":"next","body":"ship the fix","sources":[]}]}`,
		"stage=reflect":   `{"no_change_reason":"done"}`,
	}}
}

func cognitiveBinding(domain_ laputaevolution.Domain, source CognitiveSource, store storage.SnapshotStore, now func() int64) *CognitiveBinding {
	return &CognitiveBinding{
		Domain: domain_, Source: source, Store: store, SourceID: "activity",
		Policy: laputaevolution.TriggerPolicy{Enabled: true},
		Now:    now,
		Binding: laputaevolution.RunBinding{
			SubjectID: "profile-1", WorkspaceID: "ws-1", DestinationID: "mentle",
			PolicyRevision: "pol-1", StrategyDigest: "dig-1", MissionRevision: 1,
		},
	}
}

func listWorkflowRuns(t *testing.T, svc *Service, backend interface {
	ListRunsBySession(context.Context, domain.SessionID) ([]domain.Run, error)
}) []domain.Run {
	t.Helper()
	runs, err := backend.ListRunsBySession(context.Background(), "sess_cognitive_supervisor")
	if err != nil {
		t.Fatal(err)
	}
	var out []domain.Run
	for _, r := range runs {
		if r.Kind == domain.RunKindWorkflow {
			out = append(out, r)
		}
	}
	return out
}

func TestCognitiveWakeCoalescesToOneActiveRun(t *testing.T) {
	ctx := context.Background()
	gate := make(chan struct{})
	d := &fakeCognitiveDomain{gate: gate, batch: laputaevolution.EvidenceBatch{ActivityRevision: 3}}
	svc, backend := inofyExecService(t, cognitiveTestModel())
	svc.deps.Cognitive = cognitiveBinding(d, &fakeSource{high: 5}, backend.Snapshot(), func() int64 { return 1000 })
	svc.StartCognitiveLoop(ctx, time.Hour)
	t.Cleanup(svc.StopCognitiveLoop)

	elig, err := svc.cognitiveAttempt(ctx, false)
	if err != nil {
		t.Fatal(err)
	}
	if !elig.Run {
		t.Fatalf("first wake not eligible: %+v", elig)
	}
	waitFor := func() domain.Run {
		deadline := time.Now().Add(5 * time.Second)
		for time.Now().Before(deadline) {
			if wf := listWorkflowRuns(t, svc, backend); len(wf) > 0 {
				return wf[0]
			}
			time.Sleep(10 * time.Millisecond)
		}
		t.Fatal("no workflow run created")
		return domain.Run{}
	}
	wf := waitFor()
	// While the workflow holds the domain gate, repeated wakeups see the
	// persisted active run and never spawn a second task.
	for i := 0; i < 3; i++ {
		elig, err = svc.cognitiveAttempt(ctx, false)
		if err != nil {
			t.Fatal(err)
		}
		if elig.Run {
			t.Fatalf("wake %d started a second task", i)
		}
		if elig.Reason != laputaevolution.ReasonActive {
			t.Fatalf("wake %d reason = %q, want active", i, elig.Reason)
		}
	}
	if got := listWorkflowRuns(t, svc, backend); len(got) != 1 {
		t.Fatalf("workflow runs = %d, want 1", len(got))
	}
	close(gate)
	waitForRunStatus(t, backend, wf.ID, domain.RunCompleted)
	// Settled success advances the watermark; the next wake is a no-op.
	elig, err = svc.cognitiveAttempt(ctx, false)
	if err != nil {
		t.Fatal(err)
	}
	if elig.Reason != laputaevolution.ReasonNoNewInput {
		t.Fatalf("post-settle reason = %q, want no_new_input", elig.Reason)
	}
	st, watermark, err := svc.CognitiveStatus(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if watermark != 5 || st.ActiveRunID != "" || st.LastCompletedUnixMS != 1000 {
		t.Fatalf("state after settle: %+v watermark=%d", st, watermark)
	}
}

func TestCognitiveIntentCrashMatrix(t *testing.T) {
	ctx := context.Background()
	dbPath := filepath.Join(t.TempDir(), "cognitive-crash-cuts.db")
	gate := make(chan struct{})
	var gateOnce sync.Once
	closeGate := func() { gateOnce.Do(func() { close(gate) }) }
	entered := make(chan struct{}, 1)
	d := &fakeCognitiveDomain{gate: gate, entered: entered}
	svc, backend := newCognitivePersistentService(t, dbPath, cognitiveTestModel())
	svc.deps.Cognitive = cognitiveBinding(d, &fakeSource{high: 9}, backend.Snapshot(), func() int64 { return 50 })
	originalStoreClosed := false
	var restarted *Service
	t.Cleanup(func() {
		if originalStoreClosed {
			return
		}
		closeGate()
		if restarted != nil {
			restarted.CancelAll()
			restarted.WaitIdle(context.Background())
		}
		svc.CancelAll()
		svc.WaitIdle(context.Background())
		_ = backend.Close()
	})

	elig, err := svc.TriggerCognitive(ctx)
	if err != nil || !elig.Run {
		t.Fatalf("first admission = %+v, %v", elig, err)
	}
	select {
	case <-entered:
	case <-time.After(5 * time.Second):
		t.Fatal("strategy did not reach its held Collect call")
	}

	raw, _, err := svc.deps.Cognitive.Store.Get(ctx, cognitiveStateKey)
	if err != nil {
		t.Fatal(err)
	}
	var snapshot map[string]json.RawMessage
	if err := json.Unmarshal(raw, &snapshot); err != nil {
		t.Fatal(err)
	}
	var stateSchema int
	_ = json.Unmarshal(snapshot["state_schema"], &stateSchema)
	if stateSchema != 2 {
		t.Fatalf("state_schema = %d, want 2; snapshot=%s", stateSchema, raw)
	}
	if phase := jsonStringField(t, snapshot["phase"]); phase != "running" {
		t.Fatalf("phase = %q, want running after admission", phase)
	}
	var intent struct {
		ParentRunID  string          `json:"parent_run_id"`
		OperationKey string          `json:"operation_key"`
		StrategyID   string          `json:"strategy_id"`
		Input        json.RawMessage `json:"input"`
		Attempt      int             `json:"attempt"`
	}
	if err := json.Unmarshal(snapshot["intent"], &intent); err != nil {
		t.Fatalf("decode persisted intent: %v (snapshot=%s)", err, raw)
	}
	if intent.ParentRunID == "" || intent.OperationKey == "" || intent.StrategyID != TrustedStrategyDIVA || len(intent.Input) == 0 {
		t.Fatalf("incomplete admission intent: %+v", intent)
	}
	var input laputaevolution.Input
	if err := json.Unmarshal(intent.Input, &input); err != nil {
		t.Fatal(err)
	}
	if input.Window.After != 0 || input.Window.Through != 9 || input.Window.SourceID != "activity" {
		t.Fatalf("intent window = %+v, want activity (0,9]", input.Window)
	}

	// A fresh Service instance shares only the durable stores. Reconciliation
	// must find and adopt this same workflow under its original identity.
	restarted = NewService(svc.engine, svc.provider, svc.modelID, svc.deps)
	restarted.engine.SetRetryDecider(svc.overflowRetryDecision)
	recovered, err := restarted.TriggerCognitive(ctx)
	if err != nil || recovered.Reason != laputaevolution.ReasonActive {
		t.Fatalf("restart adoption = %+v, %v", recovered, err)
	}
	if got := listWorkflowRuns(t, restarted, backend); len(got) != 1 || string(got[0].ID) != jsonStringField(t, snapshot["active_run_id"]) {
		t.Fatalf("restart created or changed workflow identity: %+v", got)
	}

	if err := svc.NotifyCognitiveInput(ctx, 10); err != nil {
		t.Fatal(err)
	}
	closeGate()
	workflows := listWorkflowRuns(t, svc, backend)
	if len(workflows) != 1 {
		t.Fatalf("admitted workflows = %d, want one", len(workflows))
	}
	waitForRunStatus(t, backend, workflows[0].ID, domain.RunCompleted)
	svc.WaitIdle(ctx)
	restarted.WaitIdle(ctx)
	if err := backend.Close(); err != nil {
		t.Fatal(err)
	}
	originalStoreClosed = true

	// The run completed, but its cognitive snapshot still carries the
	// original intent: reopen SQLite before settling that persisted window.
	reopened, reopenedBackend := newCognitivePersistentService(t, dbPath, cognitiveTestModel())
	reopened.deps.Cognitive = cognitiveBinding(d, &fakeSource{high: 9}, reopenedBackend.Snapshot(), func() int64 { return 50 })
	reopenedClosed := false
	t.Cleanup(func() {
		if reopenedClosed {
			return
		}
		reopened.StopCognitiveLoop()
		reopened.CancelAll()
		reopened.WaitIdle(context.Background())
		_ = reopenedBackend.Close()
	})
	if err := reopened.UpdateCognitivePolicy(ctx, laputaevolution.TriggerPolicy{Enabled: false}); err != nil {
		t.Fatal(err)
	}
	reopened.StartCognitiveLoop(ctx, time.Hour)
	if _, err := reopened.cognitiveAttempt(ctx, false); err != nil {
		t.Fatal(err)
	}
	reopened.StopCognitiveLoop()
	reopened.WaitIdle(ctx)
	settled, _, err := reopened.loadCognitiveState(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if settled.Watermark != 9 || settled.SourceHigh != 10 {
		t.Fatalf("settlement consumed later input: watermark=%d source_high=%d", settled.Watermark, settled.SourceHigh)
	}
	if settled.Phase != "completed" || settled.Intent != nil {
		t.Fatalf("completed window did not clear its intent: phase=%q intent=%+v", settled.Phase, settled.Intent)
	}
	if len(listWorkflowRuns(t, reopened, reopenedBackend)) != 1 {
		t.Fatal("later input was admitted despite the disabled wake policy")
	}
	if err := reopenedBackend.Close(); err != nil {
		t.Fatal(err)
	}
	reopenedClosed = true

	verified, verifiedBackend := newCognitivePersistentService(t, dbPath, cognitiveTestModel())
	verified.deps.Cognitive = cognitiveBinding(&fakeCognitiveDomain{}, &fakeSource{high: 9}, verifiedBackend.Snapshot(), func() int64 { return 50 })
	t.Cleanup(func() { verified.CancelAll(); verified.WaitIdle(context.Background()); _ = verifiedBackend.Close() })
	settledAfterReopen, _, err := verified.loadCognitiveState(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if settledAfterReopen.Watermark != 9 || settledAfterReopen.SourceHigh != 10 || settledAfterReopen.Intent != nil {
		t.Fatalf("settled snapshot did not survive a second reopen: %+v", settledAfterReopen)
	}
	if len(listWorkflowRuns(t, verified, verifiedBackend)) != 1 {
		t.Fatal("reopening a settled snapshot created another workflow")
	}
}

func TestCognitiveIntentPreservesLaterInput(t *testing.T) {
	ctx := context.Background()
	gate := make(chan struct{})
	entered := make(chan struct{}, 1)
	d := &fakeCognitiveDomain{gate: gate, entered: entered}
	svc, backend := inofyExecService(t, cognitiveTestModel())
	svc.deps.Cognitive = cognitiveBinding(d, &fakeSource{high: 9}, backend.Snapshot(), func() int64 { return 50 })
	if _, err := svc.TriggerCognitive(ctx); err != nil {
		t.Fatal(err)
	}
	select {
	case <-entered:
	case <-time.After(5 * time.Second):
		t.Fatal("strategy did not reach its held Collect call")
	}
	if err := svc.NotifyCognitiveInput(ctx, 10); err != nil {
		t.Fatal(err)
	}
	close(gate)
	runs := listWorkflowRuns(t, svc, backend)
	if len(runs) != 1 {
		t.Fatalf("admitted workflows = %d, want one", len(runs))
	}
	waitForRunStatus(t, backend, runs[0].ID, domain.RunCompleted)
	state, version, err := svc.loadCognitiveState(ctx)
	if err != nil {
		t.Fatal(err)
	}
	state, _, err = svc.reconcileCognitiveIntent(ctx, state, version)
	if err != nil {
		t.Fatal(err)
	}
	if state.Watermark != 9 || state.SourceHigh != 10 {
		t.Fatalf("completed window consumed later input: watermark=%d source_high=%d", state.Watermark, state.SourceHigh)
	}
}

func jsonStringField(t *testing.T, raw json.RawMessage) string {
	t.Helper()
	var value string
	if err := json.Unmarshal(raw, &value); err != nil {
		t.Fatal(err)
	}
	return value
}

func TestCognitiveLegacyCrashGapFencesUnknown(t *testing.T) {
	ctx := context.Background()
	svc, backend := inofyExecService(t, cognitiveTestModel())
	svc.deps.Cognitive = cognitiveBinding(&fakeCognitiveDomain{}, nil, backend.Snapshot(), func() int64 { return 50 })
	legacy, err := json.Marshal(cognitiveState{
		SourceHigh: 9,
		Policy:     laputaevolution.TriggerPolicy{Enabled: true},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.deps.Cognitive.Store.Put(ctx, cognitiveStateKey, legacy, 0); err != nil {
		t.Fatal(err)
	}

	_, err = svc.TriggerCognitive(ctx)
	if err != nil {
		t.Fatal(err)
	}
	state, _, err := svc.loadCognitiveState(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if state.Blocked != cognitiveBlockUnknown || state.Watermark != 0 {
		t.Fatalf("legacy ambiguous window was not fenced: blocked=%q watermark=%d", state.Blocked, state.Watermark)
	}
	if len(listWorkflowRuns(t, svc, backend)) != 0 {
		t.Fatal("legacy crash gap was replayed as a new workflow")
	}
}

func TestCognitiveUnknownStateSchemaRejected(t *testing.T) {
	ctx := context.Background()
	svc, backend := inofyExecService(t, cognitiveTestModel())
	svc.deps.Cognitive = cognitiveBinding(&fakeCognitiveDomain{}, nil, backend.Snapshot(), func() int64 { return 50 })
	if err := svc.deps.Cognitive.Store.Put(ctx, cognitiveStateKey,
		[]byte(`{"state_schema":99,"policy":{"enabled":true},"source_high":9}`), 0); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.TriggerCognitive(ctx); err == nil {
		t.Fatal("unknown future cognitive state schema was accepted")
	}
	if len(listWorkflowRuns(t, svc, backend)) != 0 {
		t.Fatal("unknown future state schema admitted a workflow")
	}
}

func TestCognitiveRetrySafetyLookupFailureFences(t *testing.T) {
	ctx := context.Background()
	svc, backend := inofyExecService(t, cognitiveTestModel())
	sessionID := domain.SessionID("sess-cog-retry-evidence")
	if err := backend.CreateSession(ctx, domain.Session{ID: sessionID, Title: "evidence", CreatedAt: 1}); err != nil {
		t.Fatal(err)
	}
	if err := backend.CreateRun(ctx, domain.Run{ID: "run-cog-no-revision", SessionID: sessionID,
		Status: domain.RunFailed, Kind: domain.RunKindWorkflow, ParentID: "run-cog-parent", RootID: "run-cog-parent"}); err != nil {
		t.Fatal(err)
	}
	if !svc.cognitiveRunBlocked(ctx, "run-cog-no-revision") {
		t.Fatal("failed workflow inspection without revision evidence was classified retry-safe")
	}
}

func TestCognitiveNoNewInputCallsNoModel(t *testing.T) {
	ctx := context.Background()
	model := cognitiveTestModel()
	d := &fakeCognitiveDomain{}
	svc, backend := inofyExecService(t, model)
	svc.deps.Cognitive = cognitiveBinding(d, &fakeSource{high: 0}, backend.Snapshot(), func() int64 { return 7 })
	svc.StartCognitiveLoop(ctx, time.Hour)
	t.Cleanup(svc.StopCognitiveLoop)

	elig, err := svc.cognitiveAttempt(ctx, false)
	if err != nil {
		t.Fatal(err)
	}
	if elig.Run || elig.Reason != laputaevolution.ReasonNoNewInput {
		t.Fatalf("unexpected admission: %+v", elig)
	}
	if model.calls != 0 || d.collects != 0 {
		t.Fatalf("model calls=%d collects=%d", model.calls, d.collects)
	}
}

func TestCognitiveManualFollowsAdmission(t *testing.T) {
	ctx := context.Background()
	model := cognitiveTestModel()
	d := &fakeCognitiveDomain{batch: laputaevolution.EvidenceBatch{ActivityRevision: 3}}
	svc, backend := inofyExecService(t, model)
	clock := int64(1000)
	svc.deps.Cognitive = cognitiveBinding(d, &fakeSource{high: 3}, backend.Snapshot(), func() int64 { return clock })
	svc.StartCognitiveLoop(ctx, time.Hour)
	t.Cleanup(svc.StopCognitiveLoop)

	// A live user-facing primary run postpones automatic work.
	prepareChildSessionAuthorizer(t, svc, backend, "sess-user", "run-user-active", nil)
	elig, err := svc.cognitiveAttempt(ctx, false)
	if err != nil {
		t.Fatal(err)
	}
	if elig.Run || elig.Reason != laputaevolution.ReasonForegroundBusy {
		t.Fatalf("automatic wake should be postponed: %+v", elig)
	}
	if n := len(listWorkflowRuns(t, svc, backend)); n != 0 {
		t.Fatalf("foreground busy spawned %d workflows", n)
	}
	// Manual entry bypasses the busy gate through the same admission.
	elig, err = svc.TriggerCognitive(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if !elig.Run {
		t.Fatalf("manual wake blocked: %+v", elig)
	}
	if n := len(listWorkflowRuns(t, svc, backend)); n != 1 {
		t.Fatalf("manual start created %d workflows", n)
	}
}

func TestCognitiveDisabledPolicyBlocks(t *testing.T) {
	ctx := context.Background()
	svc, backend := inofyExecService(t, cognitiveTestModel())
	svc.deps.Cognitive = cognitiveBinding(&fakeCognitiveDomain{}, &fakeSource{high: 9}, backend.Snapshot(), func() int64 { return 1 })
	svc.deps.Cognitive.Policy.Enabled = false
	svc.StartCognitiveLoop(ctx, time.Hour)
	t.Cleanup(svc.StopCognitiveLoop)

	for _, manual := range []bool{false, true} {
		elig, err := svc.cognitiveAttempt(ctx, manual)
		if err != nil {
			t.Fatal(err)
		}
		if elig.Run || elig.Reason != laputaevolution.ReasonDisabled {
			t.Fatalf("manual=%v admission = %+v", manual, elig)
		}
	}
}

func TestCognitiveFailedRunRetriesWithNewKey(t *testing.T) {
	ctx := context.Background()
	d := &fakeCognitiveDomain{
		batch: laputaevolution.EvidenceBatch{ActivityRevision: 3},
		// Poison Collect so the workflow settles failed before inference.
		collectErr: errors.New("fixture: collect exploded"),
	}
	svc, backend := inofyExecService(t, cognitiveTestModel())
	svc.deps.Cognitive = cognitiveBinding(d, &fakeSource{high: 5}, backend.Snapshot(), func() int64 { return 2 })
	svc.StartCognitiveLoop(ctx, time.Hour)
	t.Cleanup(svc.StopCognitiveLoop)

	elig, err := svc.cognitiveAttempt(ctx, false)
	if err != nil || !elig.Run {
		t.Fatalf("first wake: %v %+v", err, elig)
	}
	var first domain.Run
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) && first.ID == "" {
		if wf := listWorkflowRuns(t, svc, backend); len(wf) > 0 {
			first = wf[0]
		}
		time.Sleep(10 * time.Millisecond)
	}
	waitForRunStatus(t, backend, first.ID, domain.RunFailed)

	elig, err = svc.cognitiveAttempt(ctx, false)
	if err != nil {
		t.Fatal(err)
	}
	if !elig.Run {
		afterRetry, _, _ := svc.loadCognitiveState(ctx)
		workflows := listWorkflowRuns(t, svc, backend)
		t.Fatalf("retry after failure blocked: %+v state=%+v workflows=%+v", elig, afterRetry, workflows)
	}
	deadline = time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if wf := listWorkflowRuns(t, svc, backend); len(wf) > 1 {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("failed run was never retried under a new operation key")
}

// fakeMission supplies a scripted authority Mission revision.
type fakeMission struct{ rev uint64 }

func (f *fakeMission) MissionRevision(context.Context) (uint64, error) { return f.rev, nil }

func TestCognitiveStaleMissionBlocks(t *testing.T) {
	ctx := context.Background()
	svc, backend := inofyExecService(t, cognitiveTestModel())
	binding := cognitiveBinding(&fakeCognitiveDomain{}, &fakeSource{high: 4}, backend.Snapshot(), func() int64 { return 9 })
	binding.Binding.MissionRevision = 3
	binding.Mission = &fakeMission{rev: 7}
	svc.deps.Cognitive = binding
	svc.StartCognitiveLoop(ctx, time.Hour)
	t.Cleanup(svc.StopCognitiveLoop)

	_, err := svc.cognitiveAttempt(ctx, false)
	if laputaevolution.CodeOf(err) != laputaevolution.ErrMissionRevisionChanged {
		t.Fatalf("stale mission not blocked: %v", err)
	}
	if n := len(listWorkflowRuns(t, svc, backend)); n != 0 {
		t.Fatalf("stale mission spawned %d workflows", n)
	}
	// Repinning to the current revision re-opens admission.
	binding.Binding.MissionRevision = 7
	elig, err := svc.cognitiveAttempt(ctx, false)
	if err != nil || !elig.Run {
		t.Fatalf("fresh mission pin rejected: %v %+v", err, elig)
	}
}

func TestCognitiveCurrentMissionBindingAdmitted(t *testing.T) {
	ctx := context.Background()
	svc, backend := inofyExecService(t, cognitiveTestModel())
	binding := cognitiveBinding(&fakeCognitiveDomain{}, &fakeSource{high: 5}, backend.Snapshot(), func() int64 { return 1000 })
	binding.Binding.MissionRevision = 1
	binding.Mission = &fakeMission{rev: 2}
	binding.Resolve = func(context.Context, laputaevolution.TriggerPolicy) (laputaevolution.RunBinding, error) {
		resolved := binding.Binding
		resolved.MissionRevision = 2
		return resolved, nil
	}
	svc.deps.Cognitive = binding
	svc.StartCognitiveLoop(ctx, time.Hour)
	t.Cleanup(svc.StopCognitiveLoop)

	eligibility, err := svc.cognitiveAttempt(ctx, false)
	if err != nil {
		t.Fatalf("admit against current Mission: %v", err)
	}
	if !eligibility.Run {
		t.Fatalf("eligibility = %+v, want a run pinned to current Mission revision 2", eligibility)
	}
	state, _, err := svc.loadCognitiveState(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if state.Intent == nil {
		t.Fatal("admitted run has no durable intent")
	}
	var input laputaevolution.Input
	if err := json.Unmarshal(state.Intent.Input, &input); err != nil {
		t.Fatal(err)
	}
	if input.Binding.MissionRevision != 2 {
		t.Fatalf("intent Mission revision = %d, want current revision 2", input.Binding.MissionRevision)
	}
}

func TestCognitiveMissionAssignmentRaceFailsBeforeAdmission(t *testing.T) {
	ctx := context.Background()
	service, backend := inofyExecService(t, cognitiveTestModel())
	mission := &fakeMission{rev: 1}
	binding := cognitiveBinding(&fakeCognitiveDomain{}, &fakeSource{high: 5}, backend.Snapshot(), func() int64 { return 1000 })
	binding.Binding.MissionRevision = 0
	binding.Mission = mission
	binding.Resolve = func(ctx context.Context, _ laputaevolution.TriggerPolicy) (laputaevolution.RunBinding, error) {
		resolved := binding.Binding
		resolved.MissionRevision = mission.rev
		mission.rev = 2 // Authority changes after resolution but before admission.
		return resolved, nil
	}
	service.deps.Cognitive = binding
	service.StartCognitiveLoop(ctx, time.Hour)
	t.Cleanup(service.StopCognitiveLoop)

	_, err := service.cognitiveAttempt(ctx, false)
	if laputaevolution.CodeOf(err) != laputaevolution.ErrMissionRevisionChanged {
		t.Fatalf("mission assignment race error = %v, want mission_revision_changed", err)
	}
	if runs := listWorkflowRuns(t, service, backend); len(runs) != 0 {
		t.Fatalf("stale Mission pin admitted %d doomed workflows", len(runs))
	}
}

func TestCognitiveMissionAssignedAfterUnassignedResolutionFailsBeforeAdmission(t *testing.T) {
	ctx := context.Background()
	service, backend := inofyExecService(t, cognitiveTestModel())
	mission := &fakeMission{rev: 0}
	binding := cognitiveBinding(&fakeCognitiveDomain{}, &fakeSource{high: 5}, backend.Snapshot(), func() int64 { return 1000 })
	binding.Binding.MissionRevision = 0
	binding.Mission = mission
	binding.Resolve = func(context.Context, laputaevolution.TriggerPolicy) (laputaevolution.RunBinding, error) {
		resolved := binding.Binding // Resolver observed the unassigned revision 0.
		mission.rev = 1             // Mission assignment races admission revalidation.
		return resolved, nil
	}
	service.deps.Cognitive = binding
	service.StartCognitiveLoop(ctx, time.Hour)
	t.Cleanup(service.StopCognitiveLoop)

	_, err := service.cognitiveAttempt(ctx, false)
	if laputaevolution.CodeOf(err) != laputaevolution.ErrMissionRevisionChanged {
		t.Fatalf("unassigned Mission race error = %v, want mission_revision_changed", err)
	}
	if runs := listWorkflowRuns(t, service, backend); len(runs) != 0 {
		t.Fatalf("stale unassigned Mission pin admitted %d workflows", len(runs))
	}
}

func TestCognitiveMissionChangesAfterResolutionFailClosed(t *testing.T) {
	ctx := context.Background()
	svc, backend := inofyExecService(t, cognitiveTestModel())
	binding := cognitiveBinding(&fakeCognitiveDomain{}, &fakeSource{high: 5}, backend.Snapshot(), func() int64 { return 1000 })
	binding.Binding.MissionRevision = 1
	binding.Mission = &fakeMission{rev: 3}
	input, err := json.Marshal(laputaevolution.Input{
		Binding: func() laputaevolution.RunBinding {
			resolved := binding.Binding
			resolved.MissionRevision = 2
			return resolved
		}(),
		Window: laputaevolution.Window{SourceID: binding.SourceID, After: 0, Through: 5},
	})
	if err != nil {
		t.Fatal(err)
	}
	svc.deps.Cognitive = binding
	if _, err := svc.workflowNodes(ctx, "parent", TrustedStrategyDIVA, input); err == nil {
		t.Fatal("effect path accepted Mission revision 2 after authority advanced to revision 3")
	}
}

func TestCognitivePolicyPinSurvivesReopen(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "cognitive-state.db")
	stateStore, err := sqlite.Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if stateStore != nil {
			_ = stateStore.Close()
		}
	})

	policy := laputaevolution.TriggerPolicy{Enabled: true, MinIntervalMS: 91_000}
	seedService, _ := inofyExecService(t, cognitiveTestModel())
	seedBinding := cognitiveBinding(&fakeCognitiveDomain{}, &fakeSource{high: 0}, stateStore.Snapshot(), func() int64 { return 1_000 })
	seedService.deps.Cognitive = seedBinding
	if err := seedService.UpdateCognitivePolicyCAS(ctx, policy, 0); err != nil {
		t.Fatalf("persist policy: %v", err)
	}
	if err := stateStore.Close(); err != nil {
		t.Fatal(err)
	}
	stateStore = nil

	stateStore, err = sqlite.Open(ctx, path)
	if err != nil {
		t.Fatalf("reopen cognitive snapshot: %v", err)
	}
	service, runs := inofyExecService(t, cognitiveTestModel())
	binding := cognitiveBinding(&fakeCognitiveDomain{}, &fakeSource{high: 5}, stateStore.Snapshot(), func() int64 { return 100_000 })
	// A process-local seed deliberately differs from the stored policy. The
	// re-opened run must resolve its pin from the durable state snapshot.
	binding.Policy = laputaevolution.TriggerPolicy{}
	var resolvedPolicy laputaevolution.TriggerPolicy
	binding.Resolve = func(_ context.Context, durable laputaevolution.TriggerPolicy) (laputaevolution.RunBinding, error) {
		resolvedPolicy = durable
		resolved := binding.Binding
		resolved.PolicyRevision = "resolved-from-durable-policy"
		return resolved, nil
	}
	service.deps.Cognitive = binding
	service.StartCognitiveLoop(ctx, time.Hour)
	t.Cleanup(service.StopCognitiveLoop)

	loaded, _, err := service.CognitivePolicyState(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if loaded != policy {
		t.Fatalf("reopened policy = %+v, want %+v", loaded, policy)
	}
	eligibility, err := service.cognitiveAttempt(ctx, false)
	if err != nil {
		t.Fatalf("admission after reopen: %v", err)
	}
	if !eligibility.Run {
		t.Fatalf("reopened policy did not admit new input: %+v", eligibility)
	}
	if resolvedPolicy != policy {
		t.Fatalf("resolver policy = %+v, want the durable policy %+v", resolvedPolicy, policy)
	}
	var workflow domain.Run
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if found := listWorkflowRuns(t, service, runs); len(found) > 0 {
			workflow = found[0]
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if workflow.ID == "" {
		t.Fatal("admitted workflow was not persisted")
	}
	revision, err := runs.GetWorkflowRevision(ctx, workflow.ID)
	if err != nil {
		t.Fatal(err)
	}
	var input laputaevolution.Input
	if err := json.Unmarshal(revision.InputJSON, &input); err != nil {
		t.Fatal(err)
	}
	if input.Binding.PolicyRevision != "resolved-from-durable-policy" {
		t.Fatalf("persisted policy pin = %q", input.Binding.PolicyRevision)
	}
}

func TestCognitiveConcurrentPolicyUpdatePinsAdmissionSnapshot(t *testing.T) {
	ctx := context.Background()
	service, backend := inofyExecService(t, cognitiveTestModel())
	first := laputaevolution.TriggerPolicy{Enabled: true, MinIntervalMS: 30_000}
	second := laputaevolution.TriggerPolicy{Enabled: false, MinIntervalMS: 120_000}
	binding := cognitiveBinding(&fakeCognitiveDomain{}, &fakeSource{high: 5}, backend.Snapshot(), func() int64 { return 100_000 })
	binding.Policy = first
	service.deps.Cognitive = binding
	if err := service.UpdateCognitivePolicyCAS(ctx, first, 0); err != nil {
		t.Fatalf("persist initial policy: %v", err)
	}
	binding.Resolve = func(_ context.Context, durable laputaevolution.TriggerPolicy) (laputaevolution.RunBinding, error) {
		if durable != first {
			t.Fatalf("resolver read policy %+v, want admission snapshot %+v", durable, first)
		}
		if err := service.UpdateCognitivePolicyCAS(ctx, second, 1); err != nil {
			t.Fatalf("concurrent policy update: %v", err)
		}
		resolved := binding.Binding
		resolved.PolicyRevision = "pin-from-first-policy"
		return resolved, nil
	}

	eligibility, err := service.cognitiveAttempt(ctx, true)
	if err != nil {
		t.Fatalf("admission: %v", err)
	}
	if !eligibility.Run {
		t.Fatalf("eligibility = %+v, want admitted run", eligibility)
	}
	state, _, err := service.loadCognitiveState(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if state.Policy != second || state.PolicyRevision != 2 {
		t.Fatalf("concurrent durable policy = %+v revision=%d", state.Policy, state.PolicyRevision)
	}
	if state.Intent == nil {
		t.Fatal("durable admission intent missing")
	}
	var input laputaevolution.Input
	if err := json.Unmarshal(state.Intent.Input, &input); err != nil {
		t.Fatal(err)
	}
	if input.Binding.PolicyRevision != "pin-from-first-policy" {
		t.Fatalf("admitted run pin = %q, want the policy snapshot used by its resolver", input.Binding.PolicyRevision)
	}
}

func TestCognitiveCaptureProviderPreservesIdentity(t *testing.T) {
	ctx := context.Background()
	svc, backend, _ := newTestService(t, cognitiveTestModel())
	sink := &fakeSink{}
	var notified []CognitiveCaptureReceipt
	provider := NewCognitiveCaptureProvider(backend, sink, func(r CognitiveCaptureReceipt) {
		notified = append(notified, r)
	})

	if err := backend.CreateSession(ctx, domain.Session{ID: "sess-cap", Title: "cap", CreatedAt: 1}); err != nil {
		t.Fatal(err)
	}
	if err := backend.CreateRun(ctx, domain.Run{ID: "run_cap", SessionID: "sess-cap", Status: domain.RunCompleted, Kind: domain.RunKindPrimary}); err != nil {
		t.Fatal(err)
	}
	payload, _ := json.Marshal(map[string]string{
		"outcome": "ok", "summary": "the exchange", "tenant_id": "profile-1",
		"workspace_id": "ws-9", "session_id": "sess-cap",
	})
	event := observer.NewRunEvent(observer.NewEventID("run_cap", 4), "run.completed", 777, payload)
	receipt, err := provider.ObserveRunWithReceipt(ctx, event)
	if err != nil {
		t.Fatal(err)
	}
	if receipt.State != observer.DeliveryAccepted || len(sink.captures) != 1 {
		t.Fatalf("delivery = %+v captures=%d", receipt, len(sink.captures))
	}
	got := sink.captures[0]
	if got.SubjectID != "profile-1" || got.WorkspaceID != "ws-9" || got.SessionID != "sess-cap" ||
		got.EventID != "run_cap:4" || got.Phase != "completed" || got.Content != "the exchange" || got.OccurredAt != 777 {
		t.Fatalf("capture lost identity: %+v", got)
	}
	if len(notified) != 1 || notified[0].Seq != 1 {
		t.Fatalf("notify = %+v", notified)
	}
	// Redelivery of the same stable event ID dedupes inside the sink.
	again, err := provider.ObserveRunWithReceipt(ctx, event)
	if err != nil {
		t.Fatal(err)
	}
	if again.State != observer.DeliveryAccepted || again.ReceiptID != receipt.ReceiptID || len(sink.captures) != 1 {
		t.Fatalf("redelivery = %+v captures=%d", again, len(sink.captures))
	}
	// Workflow terminal events are strategy output — skipped but acked.
	if err := backend.CreateRun(ctx, domain.Run{ID: "run_wf", SessionID: "sess-cap", Status: domain.RunCompleted, Kind: domain.RunKindWorkflow, ParentID: "run_cap", RootID: "run_cap"}); err != nil {
		t.Fatal(err)
	}
	wfReceipt, err := provider.ObserveRunWithReceipt(ctx, observer.NewRunEvent(observer.NewEventID("run_wf", 1), "run.completed", 800, payload))
	if err != nil {
		t.Fatal(err)
	}
	if wfReceipt.State != observer.DeliveryAccepted || len(sink.captures) != 1 {
		t.Fatalf("workflow event leaked to sink: %+v captures=%d", wfReceipt, len(sink.captures))
	}
	_ = svc
}

// --- observerhost-level fixtures for the cursor test ---

type cogJournal struct {
	events map[domain.RunID][]domain.RunEvent
}

func (j *cogJournal) Append(_ context.Context, commit storage.Commit) (domain.EventSeq, error) {
	if j.events == nil {
		j.events = map[domain.RunID][]domain.RunEvent{}
	}
	seq := domain.EventSeq(len(j.events[commit.RunID]))
	for _, event := range commit.Events {
		seq++
		event.RunID = commit.RunID
		event.Seq = seq
		j.events[commit.RunID] = append(j.events[commit.RunID], event)
	}
	return seq, nil
}

func (j *cogJournal) Replay(_ context.Context, runID domain.RunID, after domain.EventSeq) (storage.Iterator[storage.Entry], error) {
	var entries []storage.Entry
	for _, event := range j.events[runID] {
		if event.Seq > after {
			entries = append(entries, storage.Entry{Event: event})
		}
	}
	return &cogIterator{entries: entries, index: -1}, nil
}

type cogIterator struct {
	entries []storage.Entry
	index   int
}

func (i *cogIterator) Next() bool           { i.index++; return i.index < len(i.entries) }
func (i *cogIterator) Value() storage.Entry { return i.entries[i.index] }
func (*cogIterator) Err() error             { return nil }
func (*cogIterator) Close() error           { return nil }

type cogSnapshots struct {
	values   map[string][]byte
	versions map[string]int64
}

func (s *cogSnapshots) Get(_ context.Context, key string) ([]byte, int64, error) {
	return append([]byte(nil), s.values[key]...), s.versions[key], nil
}
func (s *cogSnapshots) Put(_ context.Context, key string, value []byte, expected int64) error {
	if s.values == nil {
		s.values = map[string][]byte{}
		s.versions = map[string]int64{}
	}
	if s.versions[key] != expected {
		return storage.ErrVersionConflict
	}
	s.values[key] = append([]byte(nil), value...)
	s.versions[key]++
	return nil
}

type cogRuns struct{ runs map[domain.RunID]domain.Run }

func (r *cogRuns) CreateRun(_ context.Context, run domain.Run) error {
	if r.runs == nil {
		r.runs = map[domain.RunID]domain.Run{}
	}
	r.runs[run.ID] = run
	return nil
}
func (r *cogRuns) GetRun(_ context.Context, id domain.RunID) (domain.Run, error) {
	run, ok := r.runs[id]
	if !ok {
		return domain.Run{}, storage.ErrNotFound
	}
	return run, nil
}
func (r *cogRuns) SetRunStatus(_ context.Context, id domain.RunID, status domain.RunStatus) error {
	run, err := r.GetRun(context.Background(), id)
	if err != nil {
		return err
	}
	run.Status = status
	r.runs[id] = run
	return nil
}
func (r *cogRuns) ListActiveRuns(context.Context) ([]domain.Run, error) { return nil, nil }
func (r *cogRuns) ListChildRuns(context.Context, domain.RunID) ([]domain.Run, error) {
	return nil, nil
}
func (r *cogRuns) ListRunTree(context.Context, domain.RunID) ([]domain.Run, error) {
	return nil, nil
}
func (r *cogRuns) ListRunsBySession(context.Context, domain.SessionID) ([]domain.Run, error) {
	return nil, nil
}

// The capture cursor may only advance once the sink reports durable
// acceptance; a failing sink replays the same stable event ID.
func TestCognitiveCaptureCursorFollowsAcceptance(t *testing.T) {
	ctx := context.Background()
	journal := &cogJournal{}
	snapshots := &cogSnapshots{}
	runs := &cogRuns{runs: map[domain.RunID]domain.Run{
		"run_cap": {ID: "run_cap", SessionID: "sess-cap", Status: domain.RunCompleted, Kind: domain.RunKindPrimary},
	}}
	sink := &fakeSink{}
	sinkErr := &errSink{inner: sink, fail: true}
	host, err := observerhost.New(observerhost.Config{
		Journal: journal, Cursors: snapshots,
		RunSubscriptions: []observerhost.RunSubscription{
			CognitiveCaptureSubscription(runs, sinkErr, nil),
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	payload, _ := json.Marshal(map[string]string{"outcome": "ok", "summary": "exchange"})
	if _, err := journal.Append(ctx, storage.Commit{
		RunID: "run_cap",
		Events: []domain.RunEvent{{
			Type: domain.EventRunCompleted, CreatedAt: 1, Payload: payload,
		}},
	}); err != nil {
		t.Fatal(err)
	}
	if err := host.DeliverRun(ctx, "run_cap"); err == nil {
		t.Fatal("failing sink should fail delivery")
	}
	if len(sink.captures) != 0 {
		t.Fatalf("failed sink recorded captures")
	}
	sinkErr.fail = false
	if err := host.DeliverRun(ctx, "run_cap"); err != nil {
		t.Fatal(err)
	}
	if len(sink.captures) != 1 {
		t.Fatalf("capture not delivered: %d", len(sink.captures))
	}
	// A second delivery after acceptance replays nothing: the cursor
	// advanced past the committed event.
	if err := host.DeliverRun(ctx, "run_cap"); err != nil {
		t.Fatal(err)
	}
	if len(sink.captures) != 1 {
		t.Fatalf("event redelivered after cursor advance: %d", len(sink.captures))
	}
}

type errSink struct {
	inner *fakeSink
	fail  bool
}

func (e *errSink) Capture(ctx context.Context, c CognitiveCapture) (CognitiveCaptureReceipt, error) {
	if e.fail {
		return CognitiveCaptureReceipt{}, errors.New("fixture: sink unavailable")
	}
	return e.inner.Capture(ctx, c)
}

func TestCognitiveNotifyInputFeedsWindow(t *testing.T) {
	ctx := context.Background()
	d := &fakeCognitiveDomain{batch: laputaevolution.EvidenceBatch{ActivityRevision: 3}}
	svc, backend := inofyExecService(t, cognitiveTestModel())
	// No Source: NotifyCognitiveInput is the only high watermark feed.
	svc.deps.Cognitive = cognitiveBinding(d, nil, backend.Snapshot(), func() int64 { return 50 })
	svc.StartCognitiveLoop(ctx, time.Hour)
	t.Cleanup(svc.StopCognitiveLoop)

	if err := svc.NotifyCognitiveInput(ctx, 7); err != nil {
		t.Fatal(err)
	}
	elig, err := svc.cognitiveAttempt(ctx, false)
	if err != nil {
		t.Fatal(err)
	}
	if !elig.Run {
		t.Fatalf("notified input not admitted: %+v", elig)
	}
	waitForWorkflow := func() domain.Run {
		deadline := time.Now().Add(5 * time.Second)
		for time.Now().Before(deadline) {
			if wf := listWorkflowRuns(t, svc, backend); len(wf) > 0 {
				return wf[0]
			}
			time.Sleep(10 * time.Millisecond)
		}
		t.Fatal("no workflow run")
		return domain.Run{}
	}
	wf := waitForWorkflow()
	waitForRunStatus(t, backend, wf.ID, domain.RunCompleted)
	if d.lastWindow.After != 0 || d.lastWindow.Through != 7 || d.lastWindow.SourceID != "activity" {
		t.Fatalf("window = %+v", d.lastWindow)
	}
}

func TestCognitiveStateStaleVersionRejected(t *testing.T) {
	ctx := context.Background()
	d := &fakeCognitiveDomain{}
	svc, backend := inofyExecService(t, cognitiveTestModel())
	svc.deps.Cognitive = cognitiveBinding(d, nil, backend.Snapshot(), func() int64 { return 50 })

	stale, version, err := svc.loadCognitiveState(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.NotifyCognitiveInput(ctx, 9); err != nil {
		t.Fatal(err)
	}
	// This is the stale-attempt save: it must not read the latest version and
	// overwrite the accepted capture with the older state it originally read.
	if err := svc.saveCognitiveState(ctx, stale, version); !errors.Is(err, storage.ErrVersionConflict) {
		t.Fatalf("stale save error = %v, want storage.ErrVersionConflict", err)
	}
	got, _, err := svc.loadCognitiveState(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if got.SourceHigh != 9 {
		t.Fatalf("stale save lost accepted capture: SourceHigh=%d, want 9", got.SourceHigh)
	}
}

func TestCognitiveConcurrentCapturePolicyAndSettlement(t *testing.T) {
	ctx := context.Background()
	d := &fakeCognitiveDomain{}
	svc, backend := inofyExecService(t, cognitiveTestModel())
	svc.deps.Cognitive = cognitiveBinding(d, nil, backend.Snapshot(), func() int64 { return 50 })
	base, baseVersion, err := svc.loadCognitiveState(ctx)
	if err != nil {
		t.Fatal(err)
	}
	settlement := base
	settlement.Watermark = 7
	settlement.LastCompletedUnixMS = 1234
	settlement.Blocked = cognitiveBlockExhausted

	start := make(chan struct{})
	ready := make(chan struct{}, 3)
	errs := make(chan error, 3)
	transitions := []func() error{
		func() error { return svc.NotifyCognitiveInput(ctx, 9) },
		func() error {
			return svc.UpdateCognitivePolicy(ctx, laputaevolution.TriggerPolicy{Enabled: false, MinIntervalMS: 700})
		},
		func() error {
			return svc.saveCognitiveAttemptState(ctx, base, settlement, baseVersion)
		},
	}
	for _, transition := range transitions {
		go func(transition func() error) {
			ready <- struct{}{}
			<-start
			errs <- transition()
		}(transition)
	}
	for range transitions {
		<-ready
	}
	close(start)
	for range transitions {
		if err := <-errs; err != nil {
			t.Fatal(err)
		}
	}
	_, versionBeforeRebase, err := svc.loadCognitiveState(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if versionBeforeRebase <= baseVersion {
		t.Fatalf("state version before stale settlement = %d, want greater than base %d", versionBeforeRebase, baseVersion)
	}
	// Force the conflict path even if the concurrent settlement above acquired
	// the state lock first: this old version must rebase without dropping the
	// capture or policy update.
	if err := svc.saveCognitiveAttemptState(ctx, base, settlement, baseVersion); err != nil {
		t.Fatal(err)
	}

	got, versionAfterRebase, err := svc.loadCognitiveState(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if versionAfterRebase != versionBeforeRebase+1 {
		t.Fatalf("state version after stale settlement = %d, want one rebased write after %d", versionAfterRebase, versionBeforeRebase)
	}
	if got.SourceHigh != 9 || got.PolicyRevision != 1 || got.Policy.MinIntervalMS != 700 ||
		got.Watermark != 7 || got.LastCompletedUnixMS != 1234 || got.Blocked != cognitiveBlockExhausted {
		t.Fatalf("concurrent state transitions lost data: %+v", got)
	}
}

func TestCognitivePolicyCASRejectsStaleRevision(t *testing.T) {
	ctx := context.Background()
	store := &barrierCognitiveSnapshots{values: map[string][]byte{}, versions: map[string]int64{}, release: make(chan struct{})}
	service := func() *Service {
		return &Service{deps: ServiceDeps{Cognitive: cognitiveBinding(&fakeCognitiveDomain{}, nil, store, nil)}}
	}
	services := []*Service{service(), service()}
	policies := []laputaevolution.TriggerPolicy{
		{Enabled: true, MinIntervalMS: 100},
		{Enabled: false, MinIntervalMS: 200},
	}
	errs := make(chan error, 2)
	for i := range services {
		go func(i int) {
			errs <- services[i].UpdateCognitivePolicyCAS(ctx, policies[i], 0)
		}(i)
	}
	results := []error{<-errs, <-errs}
	successes, conflicts := 0, 0
	for _, err := range results {
		switch {
		case err == nil:
			successes++
		case errors.Is(err, ErrPolicyConflict):
			conflicts++
		default:
			t.Fatalf("unexpected policy CAS error: %v", err)
		}
	}
	if successes != 1 || conflicts != 1 {
		t.Fatalf("policy CAS outcomes: successes=%d conflicts=%d, want one each (errors: %v)", successes, conflicts, results)
	}
}

// barrierCognitiveSnapshots forces two independent Services to load the same
// base revision before either policy CAS continues.
type barrierCognitiveSnapshots struct {
	mu       sync.Mutex
	values   map[string][]byte
	versions map[string]int64
	reads    int
	release  chan struct{}
}

func (s *barrierCognitiveSnapshots) Get(_ context.Context, key string) ([]byte, int64, error) {
	s.mu.Lock()
	value := append([]byte(nil), s.values[key]...)
	version := s.versions[key]
	s.reads++
	if s.reads == 2 {
		close(s.release)
	}
	wait := s.reads <= 2
	s.mu.Unlock()
	if wait {
		<-s.release
	}
	return value, version, nil
}

func (s *barrierCognitiveSnapshots) Put(_ context.Context, key string, value []byte, expected int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.versions[key] != expected {
		return storage.ErrVersionConflict
	}
	s.values[key] = append([]byte(nil), value...)
	s.versions[key]++
	return nil
}
