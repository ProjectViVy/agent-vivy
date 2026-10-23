package runtime

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"testing"
	"time"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/storage"
)

type queueFailJournal struct {
	storage.Journal
	failure error
}

type queuedModelCall struct{ thinking domain.ThinkingMode }
type queuedRequestGate struct {
	entered chan queuedModelCall
	release chan struct{}
}

func (m *queuedRequestGate) Stream(ctx context.Context, _ []*domain.Message) (domain.Stream[*domain.Message], error) {
	select {
	case m.entered <- queuedModelCall{thinking: domain.ThinkingModeFromContext(ctx)}:
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	select {
	case <-m.release:
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	return &gateStream{message: &domain.Message{Role: domain.RoleAssistant, Content: "done"}}, nil
}
func awaitQueuedCall(t *testing.T, m *queuedRequestGate) queuedModelCall {
	t.Helper()
	select {
	case call := <-m.entered:
		return call
	case <-time.After(5 * time.Second):
		t.Fatal("queued model call missing")
		return queuedModelCall{}
	}
}

func TestQueuedAdmissionAtomicallyTransfersTailBeforeInference(t *testing.T) {
	ctx := context.Background()
	model := &queuedRequestGate{entered: make(chan queuedModelCall), release: make(chan struct{})}
	svc, backend := newQueueTestService(t, model)
	defer func() { svc.CancelAll(); svc.WaitIdle(ctx) }()
	mustCreateSession(t, backend, "handoff")
	if _, err := svc.Run(ctx, "handoff", "first"); err != nil {
		t.Fatal(err)
	}
	awaitQueuedCall(t, model)
	opts := RunOptions{Thinking: domain.ThinkingMode("high"), Mode: domain.RunModePlan, FileContexts: []domain.FileContext{{Path: "a.txt", Name: "a.txt", Size: 5, Content: []byte("saved")}}}
	if _, err := svc.FollowUpWithOptions(ctx, "handoff", "second", opts); err != nil {
		t.Fatal(err)
	}
	tail, err := svc.FollowUpWithOptions(ctx, "handoff", "third", RunOptions{Thinking: domain.ThinkingMode("low")})
	if err != nil {
		t.Fatal(err)
	}
	model.release <- struct{}{}
	if call := awaitQueuedCall(t, model); call.thinking != domain.ThinkingMode("high") {
		t.Fatalf("admitted thinking=%q", call.thinking)
	}
	// The new run is already in inference. A fresh process must see its entire
	// tail from the atomic admission, even if no later event ever gets written.
	fresh := NewService(svc.engine, "test", "test-model", svc.deps)
	state := fresh.QueueState(ctx, "handoff", "")
	if state.Err != nil || len(state.FollowUps) != 1 || state.FollowUps[0].ID != tail.ID || state.FollowUps[0].Request.Thinking != domain.ThinkingMode("low") {
		t.Fatalf("handoff tail=%+v", state)
	}
	msgs, err := backend.ListMessages(ctx, "handoff")
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, msg := range msgs {
		if msg.Content == "second" {
			found = len(msg.FileContexts) == 1 && string(msg.FileContexts[0].Content) == "saved"
		}
	}
	if !found {
		t.Fatal("queued context snapshot absent from admitted message")
	}
}

func TestQueueRecoveryAdmitsSavedRequestAfterRestart(t *testing.T) {
	ctx := context.Background()
	model := newGateModel()
	svc, backend := newQueueTestService(t, model)
	defer func() { close(model.release); svc.CancelAll(); svc.WaitIdle(ctx) }()
	mustCreateSession(t, backend, "restart-full")
	if err := backend.CreateRun(ctx, domain.Run{ID: "crashed", SessionID: "restart-full", Status: domain.RunActive, CreatedAt: 1}); err != nil {
		t.Fatal(err)
	}
	data, _ := json.Marshal(domain.QueuedTurnOptions{Thinking: domain.ThinkingMode("high")})
	if err := backend.Blobs().Put(ctx, "queue/request/saved", data); err != nil {
		t.Fatal(err)
	}
	marker := newEventMapper("crashed", 64<<10).build(domain.EventTurnQueued, payloadTurnQueued{QueueID: "saved", Track: domain.QueueTrackFollowUp, Text: "saved input", RequestRef: "queue/request/saved"})
	if _, err := backend.Append(ctx, storage.Commit{RunID: "crashed", Events: []domain.RunEvent{marker}}); err != nil {
		t.Fatal(err)
	}
	if err := svc.Recover(ctx); err != nil {
		t.Fatal(err)
	}
	select {
	case <-model.entered:
	case <-time.After(5 * time.Second):
		t.Fatal("restart never admitted saved request")
	}
}

func TestQueueInterruptedSteerRemainsWithdrawableAfterRestart(t *testing.T) {
	ctx := context.Background()
	svc, backend := newQueueTestService(t, newGateModel())
	mustCreateSession(t, backend, "interrupted-steer")
	if err := backend.CreateRun(ctx, domain.Run{ID: "old-steer", SessionID: "interrupted-steer", Status: domain.RunFailed, CreatedAt: 1}); err != nil {
		t.Fatal(err)
	}
	m := newEventMapper("old-steer", 64<<10)
	events := []domain.RunEvent{
		m.build(domain.EventTurnQueued, payloadTurnQueued{QueueID: "nudge", Track: domain.QueueTrackSteer, Text: "must survive"}),
		m.build(domain.EventTurnSteered, payloadTurnSteered{QueueID: "nudge", Text: "must survive"}),
		m.build(domain.EventTurnQueued, payloadTurnQueued{QueueID: "later", Track: domain.QueueTrackFollowUp, Text: "later request"}),
		m.build(domain.EventRunFailed, payloadRunFailed{Message: "crash before steer resume"}),
	}
	if _, err := backend.Append(ctx, storage.Commit{RunID: "old-steer", Events: events}); err != nil {
		t.Fatal(err)
	}
	state := svc.QueueState(ctx, "interrupted-steer", "")
	if state.Err != nil || len(state.FollowUps) != 2 || state.FollowUps[0].Text != "must survive" || state.FollowUps[1].Text != "later request" {
		t.Fatalf("lost interrupted steer: %+v", state)
	}
	if _, err := svc.ClearQueue(ctx, "interrupted-steer"); err != nil {
		t.Fatalf("clear closed carrier: %v", err)
	}
	fresh := NewService(svc.engine, "test", "test-model", svc.deps)
	if state := fresh.QueueState(ctx, "interrupted-steer", ""); state.Err != nil || len(state.FollowUps)+len(state.Steering) != 0 {
		t.Fatalf("cleared request resurrected: %+v", state)
	}
	if err := svc.DeleteSession(ctx, "interrupted-steer"); err != nil {
		t.Fatal(err)
	}
	it, err := backend.Replay(ctx, queueControlRunID("interrupted-steer"), 0)
	if err != nil {
		t.Fatal(err)
	}
	defer it.Close()
	if it.Next() || it.Err() != nil {
		t.Fatal("deleted Session retained queue controls")
	}
}

func (j queueFailJournal) Append(ctx context.Context, commit storage.Commit) (domain.EventSeq, error) {
	for _, ev := range commit.Events {
		if ev.Type == domain.EventTurnQueued {
			return 0, j.failure
		}
	}
	return j.Journal.Append(ctx, commit)
}

func TestQueueEnqueueFailureDoesNotAcknowledgeOrMutate(t *testing.T) {
	model := newGateModel()
	svc, backend := newQueueTestService(t, model)
	ctx := context.Background()
	mustCreateSession(t, backend, "failure")
	_, err := svc.Run(ctx, "failure", "first")
	if err != nil {
		t.Fatal(err)
	}
	<-model.entered
	defer func() { close(model.release); svc.CancelAll(); svc.WaitIdle(ctx) }()
	failure := errors.New("injected queue write failure")
	svc.deps.Journal = queueFailJournal{Journal: backend, failure: failure}
	if _, err := svc.FollowUp(ctx, "failure", "must not be acknowledged"); !errors.Is(err, failure) {
		t.Fatalf("enqueue error = %v", err)
	}
	if state := svc.QueueState(ctx, "failure", ""); len(state.FollowUps) != 0 {
		t.Fatalf("failed item retained: %+v", state)
	}
}

func TestQueuedRequestRestoresAllOptionsInFIFOOrder(t *testing.T) {
	model := newGateModel()
	svc, backend := newQueueTestService(t, model)
	ctx := context.Background()
	mustCreateSession(t, backend, "snapshot")
	_, err := svc.Run(ctx, "snapshot", "first")
	if err != nil {
		t.Fatal(err)
	}
	<-model.entered
	defer func() { close(model.release); svc.CancelAll(); svc.WaitIdle(ctx) }()
	options := RunOptions{Mode: domain.RunModePlan, Face: domain.FaceCode, Thinking: domain.ThinkingMode("high"),
		Attachments:  []domain.Attachment{{Name: "image.png", MimeType: "image/png", Data: []byte{1, 2, 3}}},
		FileContexts: []domain.FileContext{{Path: "notes.txt", Name: "notes.txt", Size: 5, Content: []byte("saved")}},
	}
	first, err := svc.FollowUpWithOptions(ctx, "snapshot", "first queued", options)
	if err != nil {
		t.Fatal(err)
	}
	second, err := svc.FollowUp(ctx, "snapshot", "second queued")
	if err != nil {
		t.Fatal(err)
	}
	options.Attachments[0].Data[0] = 9
	fresh := NewService(svc.engine, "test", "test-model", svc.deps)
	state := fresh.QueueState(ctx, "snapshot", "")
	if len(state.FollowUps) != 2 || state.FollowUps[0].ID != first.ID || state.FollowUps[1].ID != second.ID {
		t.Fatalf("FIFO snapshot = %+v", state)
	}
	if !reflect.DeepEqual(state.FollowUps[0], first) {
		t.Fatalf("request changed across replay: got %+v want %+v", state.FollowUps[0], first)
	}
	if first.Request.Attachments[0].Data[0] != 1 {
		t.Fatal("queued request aliases caller bytes")
	}
}

func TestFollowUpsNeverMergeRequests(t *testing.T) {
	model := newGateModel()
	svc, backend := newQueueTestService(t, model)
	ctx := context.Background()
	mustCreateSession(t, backend, "separate")
	_, err := svc.Run(ctx, "separate", "first")
	if err != nil {
		t.Fatal(err)
	}
	<-model.entered
	for _, text := range []string{"second", "third"} {
		if _, err := svc.FollowUp(ctx, "separate", text); err != nil {
			t.Fatal(err)
		}
	}
	close(model.release)
	if !svc.WaitIdle(ctx) {
		t.Fatal("not idle")
	}
	runs, err := backend.ListRunsBySession(ctx, "separate")
	if err != nil || len(runs) != 3 {
		t.Fatalf("runs = %d, %v, want 3", len(runs), err)
	}
	fresh := NewService(svc.engine, "test", "test-model", svc.deps)
	if state := fresh.QueueState(ctx, "separate", ""); len(state.FollowUps) != 0 {
		t.Fatalf("consumed requests recovered: %+v", state)
	}
}
