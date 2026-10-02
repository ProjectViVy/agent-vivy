package runtime

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	laputaevolution "github.com/dashimaki/laputa/evolution"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/observerhost"
	"agent-vivy/internal/storage"
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
		t.Fatalf("retry after failure blocked: %+v", elig)
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
