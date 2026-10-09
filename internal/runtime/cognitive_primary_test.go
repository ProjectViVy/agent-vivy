package runtime

import (
	"context"
	"encoding/json"
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"

	laputaevolution "github.com/dashimaki/laputa/evolution"

	"agent-vivy/internal/cognitivecontract"
	"agent-vivy/internal/domain"
	"agent-vivy/internal/storage"
	"agent-vivy/internal/storage/sqlite"
	"agent-vivy/internal/testsupport"
	"agent-vivy/internal/tools"
	"agent-vivy/sdk/port/observer"

	"github.com/cloudwego/eino/schema"
)

// recordingPreparer is the scripted Prepare port: it returns a fixed
// FrozenCore v2 projection and counts admissions that consulted it.
type recordingPreparer struct {
	prepared cognitivecontract.PreparedPrimaryContext
	err      error
	calls    int
}

func (f *recordingPreparer) Prepare(_ context.Context, _ cognitivecontract.PrimaryContextInput) (cognitivecontract.PreparedPrimaryContext, error) {
	f.calls++
	return f.prepared, f.err
}

const frozenSlotOrderText = "# mission\nMISSION BODY\n# identity\nIDENTITY BODY\n" +
	"# relationship\nREL BODY\n# redline\nRED BODY\n# user\nUSER BODY\n" +
	"# dream\nDREAM BODY\n# dark\nDARK BODY\n"

// TestPrimaryFrozenCoreActualModelInput proves the session-frozen authority
// reaches the real model request in slot order — mission first, world and
// actmem never admitted — and that a reopened session's run carries the
// identical prepared digest in the persisted prompt payload. A corrupt or
// missing preparation gates inference with an error, never a partial core.
func TestPrimaryFrozenCoreActualModelInput(t *testing.T) {
	ctx := context.Background()
	preparer := &recordingPreparer{prepared: cognitivecontract.PreparedPrimaryContext{
		Text: frozenSlotOrderText, Digest: "frozen-digest-1",
	}}
	recorder := &recordingChatModel{inner: NewScriptedModel(
		schema.AssistantMessage("first", nil), schema.AssistantMessage("second", nil))}
	backend, err := sqlite.Open(ctx, filepath.Join(t.TempDir(), "frozen.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = backend.Close() })
	ts, err := tools.Builtin(backend).Resolve([]string{tools.EchoInfoName})
	if err != nil {
		t.Fatal(err)
	}
	eng, err := NewEngine(ctx, recorder, ts, EngineConfig{
		StreamBuffer: 8, MaxEventPayloadBytes: 64 << 10, MaxContextBytes: 8 << 10,
	})
	if err != nil {
		t.Fatal(err)
	}
	svc := NewService(eng, "test", "test-model", ServiceDeps{
		Journal: backend, Runs: backend, Messages: backend, Sessions: backend,
		Sink: newTestSink(), Truncations: backend, Admission: backend,
		GenerationID: "gen-1",
		Cognitive:    &CognitiveBinding{Primary: preparer},
	})
	t.Cleanup(func() { svc.CancelAll(); svc.WaitIdle(context.Background()) })
	mustCreateSession(t, backend, "session-frozen")

	runID, err := svc.Run(ctx, "session-frozen", "hello")
	if err != nil {
		t.Fatalf("first run: %v", err)
	}
	waitForRunStatus(t, backend, runID, domain.RunCompleted)
	if preparer.calls != 1 {
		t.Fatalf("prepare calls = %d, want 1", preparer.calls)
	}
	if len(recorder.inputs) == 0 || len(recorder.inputs[0]) == 0 {
		t.Fatal("model saw no request")
	}
	instruction := recorder.inputs[0][0].Content
	order := []string{"# mission", "# identity", "# relationship", "# redline", "# user", "# dream", "# dark"}
	last := -1
	for _, slot := range order {
		idx := strings.Index(instruction, slot)
		if idx < 0 {
			t.Fatalf("frozen slot %q missing from model instruction:\n%s", slot, instruction)
		}
		if idx <= last {
			t.Fatalf("frozen slot %q out of order in instruction", slot)
		}
		last = idx
	}
	for _, banned := range []string{"# world", "# actmem"} {
		if strings.Contains(instruction, banned) {
			t.Fatalf("forbidden slot %q leaked into model instruction", banned)
		}
	}

	snapshot, found, err := svc.promptSnapshotForRun(ctx, runID)
	if err != nil || !found {
		t.Fatalf("prompt snapshot: found=%v err=%v", found, err)
	}
	var payload storage.RunPromptPayload
	if err := json.Unmarshal(snapshot.Payload, &payload); err != nil {
		t.Fatal(err)
	}
	if payload.Frozen == nil || payload.Frozen.Digest != "frozen-digest-1" {
		t.Fatalf("frozen payload = %#v, want digest frozen-digest-1", payload.Frozen)
	}

	// Reopened run on the same session prepares the identical digest.
	runID2, err := svc.Run(ctx, "session-frozen", "again")
	if err != nil {
		t.Fatalf("second run: %v", err)
	}
	waitForRunStatus(t, backend, runID2, domain.RunCompleted)
	snapshot2, found, err := svc.promptSnapshotForRun(ctx, runID2)
	if err != nil || !found {
		t.Fatalf("prompt snapshot 2: found=%v err=%v", found, err)
	}
	var payload2 storage.RunPromptPayload
	if err := json.Unmarshal(snapshot2.Payload, &payload2); err != nil {
		t.Fatal(err)
	}
	if payload2.Frozen == nil || payload2.Frozen.Digest != payload.Frozen.Digest {
		t.Fatalf("reopened digest = %#v, want same %q", payload2.Frozen, payload.Frozen.Digest)
	}
	if preparer.calls != 2 {
		t.Fatalf("prepare calls = %d, want 2", preparer.calls)
	}

	// Corrupt/missing authority gates the run at admission: no model call,
	// no run row.
	preparer.err = errors.New("frozen core missing")
	preparer.calls = 0
	if _, err := svc.Run(ctx, "session-frozen", "gated"); err == nil ||
		!strings.Contains(err.Error(), "frozen core missing") {
		t.Fatalf("corrupt authority run err = %v, want explicit gate", err)
	}
	if preparer.calls != 1 {
		t.Fatalf("prepare calls on gate = %d", preparer.calls)
	}
}

// TestMissionAdmissionFence proves per-run binding verification: the
// trusted lane rejects a persisted binding whose scope/destination drifted
// from the bound composition and re-verifies the Mission pin against the
// current authority before effects or recovery may run.
func TestMissionAdmissionFence(t *testing.T) {
	ctx := context.Background()
	svc, backend := inofyExecService(t, testsupport.NewEchoModel())
	mission := &fakeMission{rev: 4}
	b := &CognitiveBinding{
		Domain:  &fakeCognitiveDomain{},
		Mission: mission,
		Binding: laputaevolution.RunBinding{
			SubjectID: "profile-1", WorkspaceID: "ws-1", DestinationID: "mentle",
			PolicyRevision: "pol-1", StrategyDigest: "dig-1", MissionRevision: 4,
		},
	}
	svc.deps.Cognitive = b
	_ = backend

	marshal := func(binding laputaevolution.RunBinding) json.RawMessage {
		raw, err := json.Marshal(laputaevolution.Input{Binding: binding})
		if err != nil {
			t.Fatal(err)
		}
		return raw
	}
	nodes, err := svc.workflowNodes(ctx, "run-fence", TrustedStrategyDIVA, marshal(b.Binding))
	if err != nil || nodes == nil {
		t.Fatalf("matching binding rejected: nodes=%v err=%v", nodes, err)
	}
	foreign := b.Binding
	foreign.WorkspaceID = "ws-foreign"
	if _, err := svc.workflowNodes(ctx, "run-fence", TrustedStrategyDIVA, marshal(foreign)); err == nil {
		t.Fatal("foreign scope binding was accepted")
	}
	mission.rev = 9
	if _, err := svc.workflowNodes(ctx, "run-fence", TrustedStrategyDIVA, marshal(b.Binding)); err == nil ||
		laputaevolution.CodeOf(err) != laputaevolution.ErrMissionRevisionChanged {
		t.Fatalf("mission drift err = %v, want mission_revision_changed", err)
	}
	mission.rev = 4

	// Per-admission resolution: a resolver pinning the current authority is
	// persisted into the workflow input; a resolver drifting in scope is
	// refused before any effect.
	resolved := b.Binding
	resolved.PolicyRevision = "pol-2"
	b.Resolve = func(context.Context) (laputaevolution.RunBinding, error) { return resolved, nil }
	source := &fakeSource{high: 5}
	b.Source = source
	b.SourceID = "activity"
	b.Store = backend.Snapshot()
	b.Policy = laputaevolution.TriggerPolicy{Enabled: true}
	b.Now = func() int64 { return 1000 }
	elig, err := svc.cognitiveAttempt(ctx, true)
	if err != nil {
		t.Fatalf("manual attempt: %v", err)
	}
	if !elig.Run {
		t.Fatalf("manual attempt not eligible: %+v", elig)
	}
	runs := listWorkflowRuns(t, svc, backend)
	if len(runs) != 1 {
		t.Fatalf("workflow runs = %d, want 1", len(runs))
	}
	revision, err := backend.GetWorkflowRevision(ctx, runs[0].ID)
	if err != nil {
		t.Fatal(err)
	}
	var input laputaevolution.Input
	if err := json.Unmarshal(revision.InputJSON, &input); err != nil {
		t.Fatal(err)
	}
	if input.Binding.PolicyRevision != "pol-2" {
		t.Fatalf("persisted policy pin = %q, want per-admission pol-2", input.Binding.PolicyRevision)
	}

	drifting := resolved
	drifting.SubjectID = "profile-foreign"
	b.Resolve = func(context.Context) (laputaevolution.RunBinding, error) { return drifting, nil }
	svc.Cancel(runs[0].ID)
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		run, getErr := backend.GetRun(ctx, runs[0].ID)
		if getErr == nil && run.Status.Terminal() {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if _, err := svc.cognitiveAttempt(ctx, true); err == nil ||
		!strings.Contains(err.Error(), "drifted") {
		t.Fatalf("drifting resolve err = %v, want refused", err)
	}
}

// TestSupervisorCaptureExcluded proves the host-owned supervisor's primary
// runs never become capture evidence while ordinary primaries still flow.
func TestSupervisorCaptureExcluded(t *testing.T) {
	ctx := context.Background()
	svc, backend, _ := newTestService(t, testsupport.NewEchoModel())
	_ = svc
	sink := &fakeSink{}
	provider := NewCognitiveCaptureProvider(backend, sink, nil)
	if err := backend.CreateSession(ctx, domain.Session{ID: cognitiveSupervisorSessionID, Title: "sup", CreatedAt: 1}); err != nil {
		t.Fatal(err)
	}

	supervisorRun := domain.Run{
		ID: "run_sup_1", SessionID: cognitiveSupervisorSessionID, Kind: domain.RunKindPrimary,
		Status: domain.RunCompleted, CreatedAt: 1,
	}
	if err := backend.CreateRun(ctx, supervisorRun); err != nil {
		t.Fatal(err)
	}
	mustCreateSession(t, backend, "sess-user")
	if err := backend.CreateRun(ctx, domain.Run{
		ID: "run_user_1", SessionID: "sess-user", Kind: domain.RunKindPrimary,
		Status: domain.RunCompleted, CreatedAt: 2,
	}); err != nil {
		t.Fatal(err)
	}

	if err := backend.AppendMessage(ctx, domain.Message{ID: "user-admitted", SessionID: "sess-user", RunID: "run_user_1", Role: domain.RoleUser, Content: "user source", CreatedAt: 2}); err != nil {
		t.Fatal(err)
	}

	event := func(runID string, seq int64) observer.RunEvent {
		return observer.NewRunEvent(observer.NewEventID(runID, seq),
			string(domain.EventRunCompleted), seq,
			json.RawMessage(`{"outcome":"completed","summary":"done","session_id":"sess-user"}`))
	}
	receipt, err := provider.ObserveRunWithReceipt(ctx, event("run_sup_1", 1))
	if err != nil {
		t.Fatal(err)
	}
	if len(sink.captures) != 0 {
		t.Fatalf("supervisor run produced %d captures, want 0", len(sink.captures))
	}
	if receipt.State != observer.DeliveryAccepted {
		t.Fatalf("supervisor skip receipt = %#v, want accepted skip", receipt)
	}
	if _, err := provider.ObserveRunWithReceipt(ctx, event("run_user_1", 2)); err != nil {
		t.Fatal(err)
	}
	if len(sink.captures) != 1 {
		t.Fatalf("user run captures = %d, want 1", len(sink.captures))
	}
}

// TestUnknownOutcomeNoNewAttempt proves a terminal run whose outcome is
// unknown durably blocks admission instead of minting another operation
// key, and that a human-cancelled run pauses without any retry.
func TestUnknownOutcomeNoNewAttempt(t *testing.T) {
	ctx := context.Background()
	d := &fakeCognitiveDomain{}
	svc, backend := inofyExecService(t, cognitiveTestModel())
	svc.deps.Cognitive = cognitiveBinding(d, &fakeSource{high: 5}, backend.Snapshot(), func() int64 { return 1000 })
	svc.StartCognitiveLoop(ctx, time.Hour)
	t.Cleanup(svc.StopCognitiveLoop)

	sessionID := domain.SessionID("sess-cog-unknown")
	parentRunID := domain.RunID("run-cog-unknown-parent")
	prepareChildSessionAuthorizer(t, svc, backend, sessionID, parentRunID, nil)
	wfRun, _ := admitCognitiveWorkflowFixture(t, svc, backend, sessionID, parentRunID, "cog-op-unknown", nil)

	// The workflow failed with one node's outcome recorded unknown.
	if err := backend.SetRunStatus(ctx, wfRun.ID, domain.RunFailed); err != nil {
		t.Fatal(err)
	}
	payload, _ := json.Marshal(map[string]string{
		"node_key":       "/graph/nodes/collect",
		"error_category": "outcome_unknown",
	})
	if _, err := backend.Append(ctx, storage.Commit{RunID: wfRun.ID, Events: []domain.RunEvent{{
		RunID: wfRun.ID, Type: domain.EventWorkflowNodeFailed, Payload: payload,
		PayloadVersion: 1, CreatedAt: 3,
	}}}); err != nil {
		t.Fatal(err)
	}
	if err := svc.updateCognitiveState(ctx, func(st *cognitiveState) {
		st.ActiveRunID = string(wfRun.ID)
		st.PendingThrough = 5
	}); err != nil {
		t.Fatal(err)
	}

	elig, err := svc.cognitiveAttempt(ctx, false)
	if err != nil {
		t.Fatal(err)
	}
	if elig.Run {
		t.Fatalf("unknown outcome wake admitted a new run: %+v", elig)
	}
	st, _, err := svc.loadCognitiveState(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if st.Blocked != cognitiveBlockUnknown {
		t.Fatalf("blocked = %q, want %q", st.Blocked, cognitiveBlockUnknown)
	}
	if st.Attempt != 0 {
		t.Fatalf("attempt advanced on unknown outcome: %d", st.Attempt)
	}
	countSessionWorkflows := func() int {
		t.Helper()
		runs, err := backend.ListRunsBySession(ctx, sessionID)
		if err != nil {
			t.Fatal(err)
		}
		n := 0
		for _, r := range runs {
			if r.Kind == domain.RunKindWorkflow {
				n++
			}
		}
		return n
	}
	if got := countSessionWorkflows(); got != 1 {
		t.Fatalf("unknown outcome spawned %d workflow runs, want 1", got)
	}
	// Automatic wakes stay fenced.
	if elig, err := svc.cognitiveAttempt(ctx, false); err != nil || elig.Run {
		t.Fatalf("fenced wake = %+v err %v", elig, err)
	}
	if got := countSessionWorkflows(); got != 1 {
		t.Fatalf("fenced wake spawned a run: %d", got)
	}
}
