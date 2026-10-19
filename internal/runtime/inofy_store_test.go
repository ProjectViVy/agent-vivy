package runtime

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	inofy "github.com/ProjectViVy/inofy"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/storage"
	"agent-vivy/internal/storage/conformance"
	"agent-vivy/internal/storage/sqlite"
	"path/filepath"
)

func inofyStepHarness(t *testing.T, tag string) (storage.Engine, inofy.RunStore, domain.RunID) {
	t.Helper()
	engine, err := sqlite.Open(context.Background(), filepath.Join(t.TempDir(), "vivy.db"))
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { _ = engine.Close() })
	slot := conformance.Slot{Engine: engine}
	wfID, _ := conformance.WorkflowStepFixture(t, slot, tag)
	store, ok := any(newINOFYRunStore(engine)).(inofy.RunStore)
	if !ok {
		t.Fatal("adapter does not implement inofy.RunStore")
	}
	return engine, store, wfID
}

// The pinned engine restarts its transition ordinal at one when classifying
// a lost running process. Its recovery commit therefore has the same raw ID
// as its real admission commit; the host must preserve both durable records.
func TestINOFYStoreRecoveryClassificationDoesNotCollideWithAdmission(t *testing.T) {
	engine, store, wf := inofyStepHarness(t, "recovery-id")
	ctx := context.Background()
	revision, err := engine.GetWorkflowRevision(ctx, wf)
	if err != nil {
		t.Fatal(err)
	}
	ref := inofy.ExecutionRef{RunID: string(wf), Epoch: 1, ProgramDigest: revision.ProgramDigest, HostBindingID: revision.HostBindingID}
	admitData, err := json.Marshal(map[string]any{"input_digest": revision.InputDigest, "limits": json.RawMessage(revision.EffectiveLimits)})
	if err != nil {
		t.Fatal(err)
	}
	admission := inofy.RunCommit{CommitID: string(wf) + "/0/" + string(wf) + "/0/1", Events: []inofy.Event{{Kind: inofy.EventRunAdmitted, Data: admitData}}, Transition: inofy.StateTransition{Expected: "", Target: inofy.RunAdmitted}}
	first, err := store.Commit(ctx, ref, admission)
	if err != nil {
		t.Fatal(err)
	}
	start := inofy.RunCommit{CommitID: string(wf) + "/0/" + string(wf) + "/0/2", Events: []inofy.Event{{Kind: inofy.EventRunStarted}}, Transition: inofy.StateTransition{Expected: inofy.RunAdmitted, Target: inofy.RunRunning}}
	if _, err := store.Commit(ctx, ref, start); err != nil {
		t.Fatal(err)
	}
	ref.Epoch = 2
	recovery := inofy.RunCommit{CommitID: admission.CommitID, Events: []inofy.Event{{Kind: inofy.EventRunRecoveryRequired}}, Transition: inofy.StateTransition{Expected: inofy.RunRunning, Target: inofy.RunRecoveryRequired}}
	receipt, err := store.Commit(ctx, ref, recovery)
	if err != nil {
		t.Fatalf("actual interrupted classification collides with original admission: %v", err)
	}
	if receipt.FirstSequence <= first.LastSequence {
		t.Fatalf("recovery rejoined unrelated admission: %+v", receipt)
	}
	state, err := store.Load(ctx, string(wf))
	if err != nil || state.Status != inofy.RunRecoveryRequired || state.Ref.Epoch != 2 {
		t.Fatalf("recovery projection not durable: %+v %v", state, err)
	}
	replayed, err := store.Commit(ctx, ref, recovery)
	if err != nil || replayed != receipt {
		t.Fatalf("same original recovery not idempotent: %+v %v", replayed, err)
	}
	ref.Epoch = 1
	original, err := store.Commit(ctx, ref, admission)
	if err != nil || original != first {
		t.Fatalf("original admission receipt changed: %+v %v", original, err)
	}
	run, err := engine.GetRun(ctx, wf)
	if err != nil || run.Status.Terminal() {
		t.Fatalf("unknown recovery fabricated native terminal: %+v %v", run, err)
	}
}

func TestINOFYStoreCommitLoadRoundTrip(t *testing.T) {
	engine, store, wf := inofyStepHarness(t, "rt")
	ctx := context.Background()
	ref := inofy.ExecutionRef{
		RunID:         string(wf),
		Epoch:         1,
		ProgramDigest: conformance.StepDigest("program-rt"),
		HostBindingID: conformance.StepDigest("binding-rt"),
	}
	st, err := store.Load(ctx, string(wf))
	if err != nil {
		t.Fatalf("load fresh: %v", err)
	}
	if st.Status != inofy.RunAdmitted || st.Ref.ProgramDigest != ref.ProgramDigest ||
		st.InputDigest != conformance.StepDigest("input-rt") {
		t.Fatalf("fresh recovery state = %+v", st)
	}
	if st.Limits.MaxNodes != 12 {
		t.Fatalf("limits not restored: %+v", st.Limits)
	}

	// empty -> admitted
	receipt, err := store.Commit(ctx, ref, inofy.RunCommit{
		CommitID: "r1",
		Events: []inofy.Event{{Kind: inofy.EventRunAdmitted, Data: json.RawMessage(
			`{"input_digest":"` + conformance.StepDigest("input-rt") + `","limits":{"max_nodes":12,"max_attempts":1}}`)}},
		Transition: inofy.StateTransition{Expected: "", Target: inofy.RunAdmitted},
	})
	if err != nil {
		t.Fatalf("commit admitted: %v", err)
	}
	if receipt.FirstSequence != 2 {
		t.Fatalf("receipt = %+v", receipt)
	}
	// admitted -> running
	receipt, err = store.Commit(ctx, ref, inofy.RunCommit{
		CommitID:   "r2",
		Events:     []inofy.Event{{Kind: inofy.EventRunStarted}},
		Transition: inofy.StateTransition{Expected: inofy.RunAdmitted, Target: inofy.RunRunning},
	})
	if err != nil || receipt.FirstSequence != 3 {
		t.Fatalf("commit running: %+v %v", receipt, err)
	}
	// running -> running with a protected result
	receipt, err = store.Commit(ctx, ref, inofy.RunCommit{
		CommitID: "r3",
		Events: []inofy.Event{
			{Kind: inofy.EventNodeStarted, Path: "n1"},
			{Kind: inofy.EventNodeAttempt, Path: "n1", Attempt: 1},
		},
		Transition: inofy.StateTransition{Expected: inofy.RunRunning, Target: inofy.RunRunning},
	})
	if err != nil || receipt.FirstSequence != 4 || receipt.LastSequence != 5 {
		t.Fatalf("commit node events: %+v %v", receipt, err)
	}
	out := json.RawMessage(`{"result":"hello"}`)
	receipt, err = store.Commit(ctx, ref, inofy.RunCommit{
		CommitID:   "r4",
		Events:     []inofy.Event{{Kind: inofy.EventNodeCompleted, Path: "n1", Attempt: 1}},
		Results:    []inofy.ProtectedResult{{Path: "n1", Attempt: 1, Output: out}},
		Transition: inofy.StateTransition{Expected: inofy.RunRunning, Target: inofy.RunRunning},
	})
	if err != nil {
		t.Fatalf("commit result: %v", err)
	}
	// terminal
	receipt, err = store.Commit(ctx, ref, inofy.RunCommit{
		CommitID:   "r5",
		Events:     []inofy.Event{{Kind: inofy.EventRunSucceeded}},
		Transition: inofy.StateTransition{Expected: inofy.RunRunning, Target: inofy.RunSucceeded},
	})
	if err != nil {
		t.Fatalf("commit terminal: %v", err)
	}

	run, err := engine.GetRun(ctx, wf)
	if err != nil || run.Status != domain.RunCompleted {
		t.Fatalf("native run status = %+v err=%v", run, err)
	}
	it, err := engine.Replay(ctx, wf, 0)
	if err != nil {
		t.Fatalf("replay: %v", err)
	}
	defer func() { _ = it.Close() }()
	var events []domain.RunEvent
	for it.Next() {
		events = append(events, it.Value().Event)
	}
	if err := it.Err(); err != nil {
		t.Fatalf("iterate: %v", err)
	}
	var kinds []domain.EventType
	for _, e := range events {
		kinds = append(kinds, e.Type)
	}
	want := []domain.EventType{
		domain.EventRunStarted, domain.EventWorkflowAdmitted, domain.EventWorkflowStarted,
		domain.EventWorkflowNodeStarted, domain.EventWorkflowNodeAttempt,
		domain.EventWorkflowNodeCompleted, domain.EventRunCompleted,
	}
	if len(kinds) != len(want) {
		t.Fatalf("journal kinds = %v", kinds)
	}
	for i := range want {
		if kinds[i] != want[i] {
			t.Fatalf("journal[%d] = %s want %s (all: %v)", i, kinds[i], want[i], kinds)
		}
	}
	// The protected output is in blobs, not in the event payload.
	var completedPayload []byte
	for _, e := range events {
		if e.Type == domain.EventWorkflowNodeCompleted {
			completedPayload = e.Payload
		}
	}
	var decoded map[string]any
	if err := json.Unmarshal(completedPayload, &decoded); err != nil || decoded["result_digest"] == "" {
		t.Fatalf("node completed payload lacks result ref: %s err=%v", completedPayload, err)
	}
	if got := decoded["result_blob_id"]; got == "" || got == nil {
		t.Fatalf("node completed payload lacks blob ref: %s", completedPayload)
	}
	if _, found, err := engine.Blobs().Get(ctx, decoded["result_blob_id"].(string)); err != nil || !found {
		t.Fatalf("result blob missing: %v found=%v", err, found)
	}
}

// Committed step events must reach live bus subscribers: run/subscribe
// replays the journal once, then waits on the bus — nothing after the
// replay cursor ever arrives unless Commit publishes what it wrote (the
// journal stays the authority; the bus only drops, never invents).
func TestINOFYStoreCommitPublishesCommittedEvents(t *testing.T) {
	_, store, wf := inofyStepHarness(t, "pub")
	raw, ok := store.(*inofyRunStore)
	if !ok {
		t.Fatal("adapter is not *inofyRunStore")
	}
	var published []domain.RunEvent
	raw.publish = func(_ context.Context, ev domain.RunEvent) {
		published = append(published, ev)
	}
	ctx := context.Background()
	ref := inofy.ExecutionRef{
		RunID:         string(wf),
		Epoch:         1,
		ProgramDigest: conformance.StepDigest("program-pub"),
		HostBindingID: conformance.StepDigest("binding-pub"),
	}
	if _, err := store.Commit(ctx, ref, inofy.RunCommit{
		CommitID: "p1",
		Events: []inofy.Event{{Kind: inofy.EventRunAdmitted, Data: json.RawMessage(
			`{"input_digest":"` + conformance.StepDigest("input-pub") + `","limits":{"max_nodes":12,"max_attempts":1}}`)}},
		Transition: inofy.StateTransition{Expected: "", Target: inofy.RunAdmitted},
	}); err != nil {
		t.Fatalf("commit admitted: %v", err)
	}
	if _, err := store.Commit(ctx, ref, inofy.RunCommit{
		CommitID: "p2",
		Events: []inofy.Event{
			{Kind: inofy.EventRunStarted},
			{Kind: inofy.EventNodeStarted, Path: "n1"},
		},
		Transition: inofy.StateTransition{Expected: inofy.RunAdmitted, Target: inofy.RunRunning},
	}); err != nil {
		t.Fatalf("commit running: %v", err)
	}
	want := []struct {
		seq domain.EventSeq
		typ domain.EventType
	}{
		{2, domain.EventWorkflowAdmitted},
		{3, domain.EventWorkflowStarted},
		{4, domain.EventWorkflowNodeStarted},
	}
	if len(published) != len(want) {
		t.Fatalf("published %d events, want %d: %+v", len(published), len(want), published)
	}
	for i, ev := range published {
		if ev.RunID != wf || ev.Seq != want[i].seq || ev.Type != want[i].typ {
			t.Fatalf("published[%d] = %+v, want seq=%d type=%s", i, ev, want[i].seq, want[i].typ)
		}
	}
}

func TestINOFYStoreErrorMapping(t *testing.T) {
	_, store, wf := inofyStepHarness(t, "errmap")
	ctx := context.Background()
	ref := inofy.ExecutionRef{
		RunID: string(wf), Epoch: 1,
		ProgramDigest: conformance.StepDigest("program-errmap"),
		HostBindingID: conformance.StepDigest("binding-errmap"),
	}
	admit := inofy.RunCommit{
		CommitID: "m1",
		Events: []inofy.Event{{Kind: inofy.EventRunAdmitted, Data: json.RawMessage(
			`{"input_digest":"` + conformance.StepDigest("input-errmap") + `","limits":{"max_nodes":12,"max_attempts":1}}`)}},
		Transition: inofy.StateTransition{Expected: "", Target: inofy.RunAdmitted},
	}
	if _, err := store.Commit(ctx, ref, admit); err != nil {
		t.Fatalf("admit: %v", err)
	}
	// same commit id, different body -> idempotency conflict
	bad := admit
	bad.Events = append(admit.Events, inofy.Event{Kind: inofy.EventRunStarted})
	if _, err := store.Commit(ctx, ref, bad); !isINOFYCode(err, inofy.ErrIdempotencyConflict) {
		t.Fatalf("want idempotency_conflict, got %v", err)
	}
	// replay identical commit -> same receipt
	if _, err := store.Commit(ctx, ref, admit); err != nil {
		t.Fatalf("identical replay: %v", err)
	}
	// a newer writer claims epoch 5
	claimed := ref
	claimed.Epoch = 5
	if _, err := store.Commit(ctx, claimed, inofy.RunCommit{
		CommitID:   "m2",
		Events:     []inofy.Event{{Kind: inofy.EventRunStarted}},
		Transition: inofy.StateTransition{Expected: inofy.RunAdmitted, Target: inofy.RunRunning},
	}); err != nil {
		t.Fatalf("epoch claim: %v", err)
	}
	// stale epoch
	stale := ref
	stale.Epoch = 3
	_, err := store.Commit(ctx, stale, inofy.RunCommit{
		CommitID:   "m3",
		Transition: inofy.StateTransition{Expected: inofy.RunRunning, Target: inofy.RunRunning},
	})
	if !isINOFYCode(err, inofy.ErrStaleWriter) {
		t.Fatalf("want stale_writer, got %v", err)
	}
	// state mismatch -> revision conflict
	_, err = store.Commit(ctx, claimed, inofy.RunCommit{
		CommitID:   "m4",
		Transition: inofy.StateTransition{Expected: inofy.RunAdmitted, Target: inofy.RunRunning},
	})
	if !isINOFYCode(err, inofy.ErrRevisionConflict) {
		t.Fatalf("want revision_conflict, got %v", err)
	}
	// unknown run -> revision conflict
	if _, err := store.Load(ctx, "workflow-no-such-run"); !isINOFYCode(err, inofy.ErrRevisionConflict) {
		t.Fatalf("want revision_conflict for unknown run, got %v", err)
	}
	// program identity mismatch -> checkpoint incompatible
	wrong := claimed
	wrong.ProgramDigest = conformance.StepDigest("other")
	_, err = store.Commit(ctx, wrong, inofy.RunCommit{
		CommitID:   "m5",
		Transition: inofy.StateTransition{Expected: inofy.RunRunning, Target: inofy.RunRunning},
	})
	if !isINOFYCode(err, inofy.ErrCheckpointIncompatible) {
		t.Fatalf("want checkpoint_incompatible, got %v", err)
	}
}

func TestINOFYStoreCheckpointBlobRoundTrip(t *testing.T) {
	engine, store, wf := inofyStepHarness(t, "cp")
	ctx := context.Background()
	ref := inofy.ExecutionRef{
		RunID: string(wf), Epoch: 1,
		ProgramDigest: conformance.StepDigest("program-cp"),
		HostBindingID: conformance.StepDigest("binding-cp"),
	}
	admit := inofy.RunCommit{
		CommitID: "c1",
		Events: []inofy.Event{{Kind: inofy.EventRunAdmitted, Data: json.RawMessage(
			`{"input_digest":"` + conformance.StepDigest("input-cp") + `","limits":{"max_nodes":12,"max_attempts":1}}`)}},
		Transition: inofy.StateTransition{Expected: "", Target: inofy.RunAdmitted},
	}
	if _, err := store.Commit(ctx, ref, admit); err != nil {
		t.Fatalf("admit: %v", err)
	}
	if _, err := store.Commit(ctx, ref, inofy.RunCommit{
		CommitID:   "c2",
		Events:     []inofy.Event{{Kind: inofy.EventRunStarted}},
		Transition: inofy.StateTransition{Expected: inofy.RunAdmitted, Target: inofy.RunRunning},
	}); err != nil {
		t.Fatalf("start: %v", err)
	}
	payload := []byte("eino-checkpoint-bytes")
	envelope := &inofy.CheckpointEnvelope{
		ContinuationGeneration: 1,
		DefinitionDigest:       conformance.StepDigest("descriptor-cp"),
		ProgramDigest:          ref.ProgramDigest,
		EinoBuild:              "v0.9.13",
		SerializerVersion:      "1",
		InputDigest:            conformance.StepDigest("input-cp"),
		Payload:                payload,
	}
	if _, err := store.Commit(ctx, ref, inofy.RunCommit{
		CommitID: "c3",
		Events: []inofy.Event{
			{Kind: inofy.EventNodeAttempt, Path: "n1", Attempt: 1},
			{Kind: inofy.EventRunWaiting, Data: json.RawMessage(
				`{"waits":[{"request_id":"w1","kind":"question","continuation_ref":"k1"}],"interrupts":{"w1":"i1"},"gates":[]}`)},
		},
		Checkpoint: envelope,
		Transition: inofy.StateTransition{Expected: inofy.RunRunning, Target: inofy.RunWaiting},
	}); err != nil {
		t.Fatalf("waiting commit: %v", err)
	}
	st, err := store.Load(ctx, string(wf))
	if err != nil {
		t.Fatalf("load waiting: %v", err)
	}
	if st.Status != inofy.RunWaiting || st.LatestCheckpoint == nil {
		t.Fatalf("state = %+v", st)
	}
	if string(st.LatestCheckpoint.Payload) != string(payload) || st.LatestCheckpoint.ContinuationGeneration != 1 {
		t.Fatalf("checkpoint payload not restored: %+v", st.LatestCheckpoint)
	}
	if len(st.Waits) != 1 || st.Waits[0].RequestID != "w1" || st.Interrupts["w1"] != "i1" {
		t.Fatalf("waits/interrupts = %+v", st)
	}
	if len(st.UnresolvedOperations) != 1 || st.UnresolvedOperations[0].Path != "n1" {
		t.Fatalf("unresolved = %+v", st.UnresolvedOperations)
	}
	// Corrupt the checkpoint payload blob: Load must fail, not return a
	// half-valid state.
	stated, err := store.Load(ctx, string(wf))
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	_ = stated
	// find blob id via storage-level load
	ss := engine.(storage.WorkflowStepStore)
	sst, err := ss.LoadWorkflowStep(ctx, wf)
	if err != nil || sst.Projection == nil {
		t.Fatalf("storage load: %v", err)
	}
	blobID := sst.Projection.CheckpointBlobID
	if err := engine.Blobs().Delete(ctx, blobID); err != nil {
		t.Fatalf("delete checkpoint blob: %v", err)
	}
	if _, err := store.Load(ctx, string(wf)); err == nil {
		t.Fatal("load returned state with a missing checkpoint blob")
	}
}

func isINOFYCode(err error, code inofy.ErrorCode) bool {
	var e *inofy.Error
	if !errors.As(err, &e) {
		return false
	}
	return e.Code == code
}
