package runtime

import (
	"agent-vivy/internal/domain"
	"agent-vivy/internal/storage"
	"agent-vivy/internal/storage/sqlite"
	"agent-vivy/internal/testsupport"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestQueueReplayReenqueueReactivatesInOrder(t *testing.T) {
	svc, backend := newQueueTestService(t, testsupport.NewEchoModel())
	ctx := context.Background()
	mustCreateSession(t, backend, "queue-fold")
	if err := backend.CreateRun(ctx, domain.Run{ID: "fold-run", SessionID: "queue-fold", Status: domain.RunActive}); err != nil {
		t.Fatal(err)
	}
	m := newEventMapper("fold-run", 64<<10)
	events := []domain.RunEvent{
		m.build(domain.EventTurnQueued, payloadTurnQueued{QueueID: "a", Track: domain.QueueTrackSteer, Text: "old"}),
		m.build(domain.EventTurnQueued, payloadTurnQueued{QueueID: "b", Track: domain.QueueTrackFollowUp, Text: "second"}),
		m.build(domain.EventTurnDequeued, payloadTurnDequeued{QueueID: "a"}),
		m.build(domain.EventTurnQueued, payloadTurnQueued{QueueID: "a", Track: domain.QueueTrackFollowUp, Text: "reactivated"}),
		m.build(domain.EventTurnQueued, payloadTurnQueued{QueueID: "c", Track: domain.QueueTrackFollowUp, Text: "third"}),
		m.build(domain.EventTurnQueued, payloadTurnQueued{QueueID: "b", Track: domain.QueueTrackFollowUp, Text: "updated"}),
	}
	if _, err := backend.Append(ctx, storage.Commit{RunID: "fold-run", Events: events}); err != nil {
		t.Fatal(err)
	}
	state := mustQueueState(t, svc, ctx, "queue-fold", "")
	var texts []string
	for _, item := range state.FollowUps {
		texts = append(texts, item.Text)
	}
	if !reflect.DeepEqual(texts, []string{"updated", "reactivated", "third"}) {
		t.Fatalf("ordered active fold = %v", texts)
	}
}

type failingQueueJournal struct {
	storage.Journal
	appendErr error
}

func (j failingQueueJournal) Append(ctx context.Context, c storage.Commit) (domain.EventSeq, error) {
	return 0, j.appendErr
}

func TestQueueEnqueueStorageFailureLeavesNoVisibleItem(t *testing.T) {
	svc, backend := newQueueTestService(t, testsupport.NewEchoModel())
	ctx := context.Background()
	mustCreateSession(t, backend, "queue-fail")
	if err := backend.CreateRun(ctx, domain.Run{ID: "queue-run", SessionID: "queue-fail", Status: domain.RunActive}); err != nil {
		t.Fatal(err)
	}
	svc.active["queue-run"] = func() {}
	svc.runSessions["queue-run"] = "queue-fail"
	want := errors.New("queue disk unavailable")
	svc.deps.Journal = failingQueueJournal{Journal: backend, appendErr: want}
	if _, err := svc.FollowUp(ctx, "queue-fail", "lost"); !errors.Is(err, want) {
		t.Fatalf("enqueue error = %v, want %v", err, want)
	}
	if state := mustQueueState(t, svc, ctx, "queue-fail", ""); len(state.FollowUps) > 0 {
		t.Fatalf("failed enqueue visible: %+v", state)
	}
}

type interruptedQueueReplay struct {
	storage.Journal
	failed bool
}

func (j *interruptedQueueReplay) Replay(ctx context.Context, id domain.RunID, after domain.EventSeq) (storage.Iterator[storage.Entry], error) {
	it, err := j.Journal.Replay(ctx, id, after)
	if err != nil {
		return nil, err
	}
	if j.failed {
		return it, nil
	}
	j.failed = true
	return &queueReplayFailure{Iterator: it}, nil
}

type queueReplayFailure struct {
	storage.Iterator[storage.Entry]
}

func (it *queueReplayFailure) Next() bool { return false }
func (it *queueReplayFailure) Err() error { return errors.New("replay interrupted") }
func TestQueueReplayFailureCanRetryWithoutPublishingPartialState(t *testing.T) {
	svc, backend := newQueueTestService(t, testsupport.NewEchoModel())
	ctx := context.Background()
	mustCreateSession(t, backend, "queue-retry")
	if err := backend.CreateRun(ctx, domain.Run{ID: "retry-run", SessionID: "queue-retry", Status: domain.RunActive}); err != nil {
		t.Fatal(err)
	}
	m := newEventMapper("retry-run", 64<<10)
	if _, err := backend.Append(ctx, storage.Commit{RunID: "retry-run", Events: []domain.RunEvent{m.build(domain.EventTurnQueued, payloadTurnQueued{QueueID: "q", Track: domain.QueueTrackFollowUp, Text: "retry"})}}); err != nil {
		t.Fatal(err)
	}
	svc.deps.Journal = &interruptedQueueReplay{Journal: backend}
	if _, err := svc.QueueState(ctx, "queue-retry", ""); err == nil {
		t.Fatal("replay failure not returned")
	}
	if state := mustQueueState(t, svc, ctx, "queue-retry", ""); len(state.FollowUps) != 1 {
		t.Fatalf("retry lost pending queue: %+v", state)
	}
}

func mustQueueState(t *testing.T, s *Service, ctx context.Context, sid domain.SessionID, after domain.RunID) QueueState {
	t.Helper()
	state, err := s.QueueState(ctx, sid, after)
	if err != nil {
		t.Fatal(err)
	}
	return state
}

type queueAdmissionSink struct {
	*testSink
	journal storage.Journal
	checked chan bool
	count   int
}

func (s *queueAdmissionSink) Publish(ev domain.RunEvent) {
	s.testSink.Publish(ev)
	if ev.Type != domain.EventRunStarted {
		return
	}
	s.count++
	if s.count != 2 {
		return
	}
	it, err := s.journal.Replay(context.Background(), ev.RunID, 0)
	if err != nil {
		s.checked <- false
		return
	}
	defer it.Close()
	found := false
	for it.Next() {
		if it.Value().Event.Type == domain.EventTurnDequeued {
			found = true
		}
	}
	s.checked <- found && it.Err() == nil
}
func TestQueueAdmissionMarkersExistBeforeRunStartedPublished(t *testing.T) {
	model := newGateModel()
	svc, backend := newQueueTestService(t, model)
	ctx := context.Background()
	mustCreateSession(t, backend, "pre-drive")
	sink := &queueAdmissionSink{testSink: newTestSink(), journal: backend, checked: make(chan bool, 1)}
	svc.deps.Sink = sink
	if _, err := svc.Run(ctx, "pre-drive", "first"); err != nil {
		t.Fatal(err)
	}
	<-model.entered
	if _, err := svc.FollowUp(ctx, "pre-drive", "next"); err != nil {
		t.Fatal(err)
	}
	close(model.release)
	select {
	case persisted := <-sink.checked:
		if !persisted {
			t.Fatal("run.started visible before durable queue admission")
		}
	case <-time.After(10 * time.Second):
		t.Fatal("follow-up not admitted")
	}
	svc.WaitIdle(ctx)
}

func TestSteerConsumptionFailureRetainsDurableQueue(t *testing.T) {
	svc, backend := newQueueTestService(t, testsupport.NewEchoModel())
	ctx := context.Background()
	mustCreateSession(t, backend, "steer-fail")
	if err := backend.CreateRun(ctx, domain.Run{ID: "steer-fail-run", SessionID: "steer-fail", Status: domain.RunActive}); err != nil {
		t.Fatal(err)
	}
	item, err := svc.enqueueTurn(ctx, "steer-fail", "steer-fail-run", domain.QueuedTurn{Text: "keep me"}, domain.QueueTrackSteer)
	if err != nil {
		t.Fatal(err)
	}
	svc.deps.Journal = failingQueueJournal{Journal: backend, appendErr: errors.New("disk failed")}
	if items, err := svc.takeSteerTurn("steer-fail", "steer-fail-run"); err == nil || len(items) > 0 {
		t.Fatal("failed steer delivered")
	}
	if got := mustQueueState(t, svc, ctx, "steer-fail", ""); len(got.Steering) != 1 || got.Steering[0].ID != item.ID {
		t.Fatalf("failed steer queue=%+v", got)
	}
}

func TestSteerTranscriptFailureDoesNotConsumeMarker(t *testing.T) {
	svc, backend := newQueueTestService(t, testsupport.NewEchoModel())
	ctx := context.Background()
	mustCreateSession(t, backend, "steer-message-fail")
	if err := backend.CreateRun(ctx, domain.Run{ID: "steer-message-run", SessionID: "steer-message-fail", Status: domain.RunActive}); err != nil {
		t.Fatal(err)
	}
	item, err := svc.enqueueTurn(ctx, "steer-message-fail", "steer-message-run", domain.QueuedTurn{Text: "keep"}, domain.QueueTrackSteer)
	if err != nil {
		t.Fatal(err)
	}
	if err := backend.AppendMessage(ctx, domain.Message{ID: "steer-" + item.ID, SessionID: "steer-message-fail", Role: domain.RoleUser, Content: "conflicting row"}); err != nil {
		t.Fatal(err)
	}
	if items, err := svc.takeSteerTurn("steer-message-fail", "steer-message-run"); err == nil || len(items) > 0 {
		t.Fatal("transcript failure consumed steer")
	}
	recovered := NewService(svc.engine, "test", "test-model", ServiceDeps{Journal: backend, Runs: backend, Messages: backend, Sink: newTestSink()})
	if got := mustQueueState(t, recovered, ctx, "steer-message-fail", ""); len(got.Steering) != 1 {
		t.Fatalf("transcript failure removed durable item: %+v", got)
	}
}

func TestQueueFullOptionsSurviveReplayAndContiguousDrain(t *testing.T) {
	model := newGateModel()
	svc, backend := newQueueTestService(t, model)
	ctx := context.Background()
	mustCreateSession(t, backend, "queue-options")
	if _, err := svc.Run(ctx, "queue-options", "initial"); err != nil {
		t.Fatal(err)
	}
	<-model.entered
	input := domain.QueuedTurn{Text: "first", Mode: domain.RunModeNormal, Thinking: domain.ThinkingLevelHigh, Face: domain.FaceWeb, Attachments: []domain.Attachment{{Name: "a.png", MimeType: "image/png", Data: []byte("original")}}, ContextPaths: []string{"a.txt"}, FileContexts: []domain.FileContext{{Path: "a.txt", Name: "a.txt", Size: 8, Content: []byte("snapshot")}}}
	if _, err := svc.FollowUpWithOptions(ctx, "queue-options", input); err != nil {
		t.Fatal(err)
	}
	input.Attachments[0].Data[0] = 'X'
	input.FileContexts[0].Content[0] = 'X'
	input.ContextPaths[0] = "changed"
	state := mustQueueState(t, svc, ctx, "queue-options", "")
	if string(state.FollowUps[0].Attachments[0].Data) != "original" || string(state.FollowUps[0].FileContexts[0].Content) != "snapshot" {
		t.Fatal("enqueue aliases caller buffers")
	}
	state.FollowUps[0].Attachments[0].Data[0] = 'Y'
	if got := mustQueueState(t, svc, ctx, "queue-options", ""); string(got.FollowUps[0].Attachments[0].Data) != "original" {
		t.Fatal("snapshot aliases durable queue")
	}
	fresh := NewService(svc.engine, "test", "test-model", ServiceDeps{Journal: backend, Runs: backend, Messages: backend, Sink: newTestSink()})
	recovered := mustQueueState(t, fresh, ctx, "queue-options", "").FollowUps[0]
	if recovered.Thinking != domain.ThinkingLevelHigh || recovered.ContextPaths[0] != "a.txt" || string(recovered.FileContexts[0].Content) != "snapshot" {
		t.Fatalf("recovered options: %+v", recovered)
	}
	for _, item := range []domain.QueuedTurn{{Text: "second", Thinking: domain.ThinkingModeOff}, {Text: "third", Thinking: domain.ThinkingLevelHigh}} {
		if _, err := svc.FollowUpWithOptions(ctx, "queue-options", item); err != nil {
			t.Fatal(err)
		}
	}
	close(model.release)
	waitFor(t, "four option-specific runs", func() bool {
		runs, _ := backend.ListRunsBySession(ctx, "queue-options")
		return len(runs) == 4 && runs[3].Status.Terminal()
	})
	svc.WaitIdle(ctx)
	messages, err := backend.ListMessages(ctx, "queue-options")
	if err != nil {
		t.Fatal(err)
	}
	var users []domain.Message
	for _, msg := range messages {
		if msg.Role == domain.RoleUser {
			users = append(users, msg)
		}
	}
	if len(users) != 4 || users[1].Content != "first" || users[2].Content != "second" || users[3].Content != "third" {
		t.Fatalf("option FIFO = %+v", users)
	}
	if string(users[1].Attachments[0].Data) != "original" || string(users[1].FileContexts[0].Content) != "snapshot" {
		t.Fatal("drain lost captured attachments/context")
	}
}

type queueFailingPrimaryStore struct {
	storage.PrimaryRunStore
	failed chan error
}

func (s queueFailingPrimaryStore) CommitPrimaryRun(ctx context.Context, in storage.PrimaryRunCommit) (domain.RunEvent, error) {
	if len(in.Events) > 0 {
		in.Events = append([]domain.RunEvent(nil), in.Events...)
		in.Events[0].Payload = nil
	}
	ev, err := s.PrimaryRunStore.CommitPrimaryRun(ctx, in)
	if len(in.Events) > 0 {
		s.failed <- err
	}
	return ev, err
}
func TestQueueAdmissionEventFailureRollsBackBeforeDrive(t *testing.T) {
	model := newGateModel()
	svc, backend := newQueueTestService(t, model)
	ctx := context.Background()
	mustCreateSession(t, backend, "admission-fail")
	failed := make(chan error, 1)
	svc.deps.PrimaryRuns = queueFailingPrimaryStore{PrimaryRunStore: backend, failed: failed}
	if _, err := svc.Run(ctx, "admission-fail", "initial"); err != nil {
		t.Fatal(err)
	}
	<-model.entered
	if _, err := svc.FollowUp(ctx, "admission-fail", "pending"); err != nil {
		t.Fatal(err)
	}
	close(model.release)
	select {
	case err := <-failed:
		if err == nil {
			t.Fatal("queue event failure was acknowledged")
		}
	case <-time.After(10 * time.Second):
		t.Fatal("admission not attempted")
	}
	svc.WaitIdle(ctx)
	runs, err := backend.ListRunsBySession(ctx, "admission-fail")
	if err != nil || len(runs) != 1 {
		t.Fatalf("failed admission visible: %+v %v", runs, err)
	}
	model.mu.Lock()
	calls := len(model.inputs)
	model.mu.Unlock()
	if calls != 1 {
		t.Fatalf("failed admission drove model: %d calls", calls)
	}
	if state := mustQueueState(t, svc, ctx, "admission-fail", ""); len(state.FollowUps) != 1 {
		t.Fatalf("failed admission lost queue: %+v", state)
	}
	fresh := NewService(svc.engine, "test", "test-model", ServiceDeps{Journal: backend, Runs: backend, Messages: backend, Sink: newTestSink()})
	if state := mustQueueState(t, fresh, ctx, "admission-fail", ""); len(state.FollowUps) != 1 {
		t.Fatalf("rollback lost durable tail: %+v", state)
	}
}
func TestCancelCommitsQueueDequeuesBeforeTerminal(t *testing.T) {
	model := newGateModel()
	svc, backend := newQueueTestService(t, model)
	ctx := context.Background()
	mustCreateSession(t, backend, "cancel-queue")
	rid, err := svc.Run(ctx, "cancel-queue", "initial")
	if err != nil {
		t.Fatal(err)
	}
	<-model.entered
	if _, err = svc.FollowUp(ctx, "cancel-queue", "return draft"); err != nil {
		t.Fatal(err)
	}
	if !svc.Cancel(rid) {
		t.Fatal("cancel not accepted")
	}
	svc.WaitIdle(ctx)
	found := false
	for _, ev := range journalEvents(t, backend, rid) {
		if ev.Type == domain.EventTurnDequeued {
			found = true
		}
		if ev.Type.Terminal() && !found {
			t.Fatal("terminal sealed before queue dequeue")
		}
	}
	if !found {
		t.Fatal("cancel lost dequeue marker")
	}
	fresh := NewService(svc.engine, "test", "test-model", ServiceDeps{Journal: backend, Runs: backend, Messages: backend, Sink: newTestSink()})
	if state := mustQueueState(t, fresh, ctx, "cancel-queue", ""); len(state.FollowUps)+len(state.Steering) > 0 {
		t.Fatalf("cancelled queue resurrected: %+v", state)
	}
}

type gatedQueueReplay struct {
	storage.Journal
	reached, release chan struct{}
	once             sync.Once
}

func (j *gatedQueueReplay) Replay(ctx context.Context, rid domain.RunID, after domain.EventSeq) (storage.Iterator[storage.Entry], error) {
	it, err := j.Journal.Replay(ctx, rid, after)
	if err != nil {
		return nil, err
	}
	j.once.Do(func() { close(j.reached); <-j.release })
	return it, nil
}
func TestConcurrentQueueReadersWaitForCompleteReplay(t *testing.T) {
	svc, backend := newQueueTestService(t, testsupport.NewEchoModel())
	ctx := context.Background()
	mustCreateSession(t, backend, "concurrent-replay")
	if err := backend.CreateRun(ctx, domain.Run{ID: "concurrent-run", SessionID: "concurrent-replay", Status: domain.RunActive}); err != nil {
		t.Fatal(err)
	}
	m := newEventMapper("concurrent-run", 64<<10)
	if _, err := backend.Append(ctx, storage.Commit{RunID: "concurrent-run", Events: []domain.RunEvent{m.build(domain.EventTurnQueued, payloadTurnQueued{QueueID: "q", Text: "pending", Track: domain.QueueTrackFollowUp})}}); err != nil {
		t.Fatal(err)
	}
	gate := &gatedQueueReplay{Journal: backend, reached: make(chan struct{}), release: make(chan struct{})}
	svc.deps.Journal = gate
	first := make(chan QueueState, 1)
	second := make(chan QueueState, 1)
	errs := make(chan error, 2)
	go func() { state, err := svc.QueueState(ctx, "concurrent-replay", ""); first <- state; errs <- err }()
	<-gate.reached
	go func() { state, err := svc.QueueState(ctx, "concurrent-replay", ""); second <- state; errs <- err }()
	select {
	case state := <-second:
		close(gate.release)
		t.Fatalf("reader observed unfinished replay: %+v", state)
	case <-time.After(50 * time.Millisecond):
	}
	close(gate.release)
	for _, ch := range []chan QueueState{first, second} {
		state := <-ch
		if len(state.FollowUps) != 1 {
			t.Fatalf("replay incomplete: %+v", state)
		}
	}
	for range 2 {
		if err := <-errs; err != nil {
			t.Fatal(err)
		}
	}
}
func TestSteerMissingCheckpointFallsBackWithoutConsumption(t *testing.T) {
	model := newScriptedToolModel()
	svc, backend := newScriptedService(t, model)
	ctx := context.Background()
	mustCreateSession(t, backend, "missing-checkpoint")
	if _, err := svc.Run(ctx, "missing-checkpoint", "initial"); err != nil {
		t.Fatal(err)
	}
	<-model.entered
	item, err := svc.Steer(ctx, "missing-checkpoint", "fallback prompt")
	if err != nil {
		t.Fatal(err)
	}
	svc.engine.cfg.Checkpoints = nil
	close(model.release)
	waitFor(t, "fallback run completed", func() bool {
		runs, _ := backend.ListRunsBySession(ctx, "missing-checkpoint")
		return len(runs) == 2 && runs[1].Status.Terminal()
	})
	svc.WaitIdle(ctx)
	runs, _ := backend.ListRunsBySession(ctx, "missing-checkpoint")
	for _, ev := range journalEvents(t, backend, runs[0].ID) {
		if ev.Type == domain.EventTurnSteered {
			t.Fatal("unavailable checkpoint consumed steer")
		}
	}
	found := false
	for _, ev := range journalEvents(t, backend, runs[1].ID) {
		if ev.Type == domain.EventTurnDequeued {
			var p payloadTurnDequeued
			_ = json.Unmarshal(ev.Payload, &p)
			found = found || p.QueueID == item.ID
		}
	}
	if !found {
		t.Fatal("fallback did not durably admit accepted steer")
	}
}

func TestSteerResumePreservesRunContextAndRefreshesBoundaryCancel(t *testing.T) {
	model := newScriptedToolModel()
	model.resumedEntered = make(chan struct{})
	model.resumedRelease = make(chan struct{})
	svc, backend := newScriptedService(t, model)
	ctx := context.Background()
	mustCreateSession(t, backend, "repeat-steer")
	rid, err := svc.RunWithOptions(ctx, "repeat-steer", "initial", RunOptions{Thinking: domain.ThinkingLevelHigh})
	if err != nil {
		t.Fatal(err)
	}
	<-model.entered
	if _, err := svc.Steer(ctx, "repeat-steer", "first steer"); err != nil {
		t.Fatal(err)
	}
	close(model.release)
	select {
	case <-model.resumedEntered:
	case <-time.After(10 * time.Second):
		t.Fatal("resume not entered")
	}
	model.mu.Lock()
	resumed := model.resumedContext
	model.mu.Unlock()
	if domain.ThinkingModeFromContext(resumed) != domain.ThinkingLevelHigh || resumed.Value(runIDContextKey{}) != rid {
		t.Fatal("resume lost run identity/thinking context")
	}
	item, err := svc.Steer(ctx, "repeat-steer", "second steer")
	if err != nil || item.Track != domain.QueueTrackSteer {
		t.Fatalf("second steer did not use fresh handle: %+v %v", item, err)
	}
	close(model.resumedRelease)
	waitFor(t, "second resumed phase", func() bool { return model.callCount() >= 4 })
	svc.WaitIdle(ctx)
	runs, _ := backend.ListRunsBySession(ctx, "repeat-steer")
	if len(runs) != 1 || !runs[0].Status.Terminal() {
		t.Fatalf("steer spawned/lost run: %+v", runs)
	}
	count := 0
	for _, ev := range journalEvents(t, backend, rid) {
		if ev.Type == domain.EventTurnSteered {
			count++
		}
	}
	if count != 2 {
		t.Fatalf("steer continuity markers=%d", count)
	}
}

func TestPendingCancellationRetriesFailedQueueRecoveryAndCommit(t *testing.T) {
	for _, failure := range []string{"replay", "commit"} {
		t.Run(failure, func(t *testing.T) {
			svc, backend := newQueueTestService(t, testsupport.NewEchoModel())
			ctx := context.Background()
			sid := domain.SessionID("cancel-retry-" + failure)
			rid := domain.RunID("parked-" + failure)
			mustCreateSession(t, backend, sid)
			if err := backend.CreateRun(ctx, domain.Run{ID: rid, SessionID: sid, Status: domain.RunActive}); err != nil {
				t.Fatal(err)
			}
			mapper := newEventMapper(rid, 64<<10)
			svc.active[rid] = func() {}
			svc.runSessions[rid] = sid
			svc.pending[rid] = pendingRun{sessionID: sid, mapper: mapper, engine: svc.engine}
			if failure == "replay" {
				svc.deps.Journal = &interruptedQueueReplay{Journal: backend}
			} else {
				svc.deps.Journal = failingQueueJournal{Journal: backend, appendErr: errors.New("terminal unavailable")}
			}
			if !svc.Cancel(rid) {
				t.Fatal("first cancellation unavailable")
			}
			svc.mu.Lock()
			_, pending := svc.pending[rid]
			svc.mu.Unlock()
			if !pending {
				t.Fatal("failed cancellation discarded retry registration")
			}
			run, err := backend.GetRun(ctx, rid)
			if err != nil || run.Status.Terminal() {
				t.Fatalf("failed cancellation committed: %+v %v", run, err)
			}
			svc.deps.Journal = backend
			if !svc.Cancel(rid) {
				t.Fatal("retry unavailable")
			}
			run, err = backend.GetRun(ctx, rid)
			if err != nil || run.Status != domain.RunCancelled {
				t.Fatalf("retry did not cancel: %+v %v", run, err)
			}
		})
	}
}

func TestQueueAdmissionRejectsRemovedSnapshot(t *testing.T) {
	svc, backend := newQueueTestService(t, testsupport.NewEchoModel())
	ctx := context.Background()
	mustCreateSession(t, backend, "stale-snapshot")
	if err := backend.CreateRun(ctx, domain.Run{ID: "old-carrier", SessionID: "stale-snapshot", Status: domain.RunActive}); err != nil {
		t.Fatal(err)
	}
	svc.active["old-carrier"] = func() {}
	svc.runSessions["old-carrier"] = "stale-snapshot"
	item, err := svc.FollowUp(ctx, "stale-snapshot", "removed text")
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := svc.QueueRemove(ctx, "stale-snapshot", item.ID); err != nil {
		t.Fatal(err)
	}
	svc.mu.Lock()
	delete(svc.active, "old-carrier")
	svc.mu.Unlock()
	if err := backend.SetRunStatus(ctx, "old-carrier", domain.RunCompleted); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.RunWithOptions(ctx, "stale-snapshot", item.Text, RunOptions{queueItems: []domain.QueuedTurn{item}}); !errors.Is(err, errQueueChanged) {
		t.Fatalf("stale admission = %v", err)
	}
	runs, err := backend.ListRunsBySession(ctx, "stale-snapshot")
	if err != nil || len(runs) != 1 {
		t.Fatalf("removed snapshot drove a run: %+v %v", runs, err)
	}
}

type rejectQueueRemovalJournal struct {
	storage.Journal
	id string
}

func (j rejectQueueRemovalJournal) Append(ctx context.Context, commit storage.Commit) (domain.EventSeq, error) {
	for _, event := range commit.Events {
		if event.Type == domain.EventTurnDequeued {
			var payload payloadTurnDequeued
			_ = json.Unmarshal(event.Payload, &payload)
			if payload.QueueID == j.id {
				return 0, errors.New("second removal unavailable")
			}
		}
	}
	return j.Journal.Append(ctx, commit)
}
func TestClearQueueFailureIsAtomic(t *testing.T) {
	svc, backend := newQueueTestService(t, testsupport.NewEchoModel())
	ctx := context.Background()
	mustCreateSession(t, backend, "atomic-clear")
	if err := backend.CreateRun(ctx, domain.Run{ID: "clear-carrier", SessionID: "atomic-clear", Status: domain.RunActive}); err != nil {
		t.Fatal(err)
	}
	svc.active["clear-carrier"] = func() {}
	svc.runSessions["clear-carrier"] = "atomic-clear"
	first, err := svc.FollowUp(ctx, "atomic-clear", "one")
	if err != nil {
		t.Fatal(err)
	}
	second, err := svc.FollowUp(ctx, "atomic-clear", "two")
	if err != nil {
		t.Fatal(err)
	}
	svc.deps.Journal = rejectQueueRemovalJournal{Journal: backend, id: second.ID}
	if _, err := svc.ClearQueue(ctx, "atomic-clear"); err == nil {
		t.Fatal("clear failure ignored")
	}
	if state := mustQueueState(t, svc, ctx, "atomic-clear", ""); len(state.FollowUps) != 2 {
		t.Fatalf("partial clear visible: %+v", state)
	}
	svc.dropSessionQueue("atomic-clear")
	state := mustQueueState(t, svc, ctx, "atomic-clear", "")
	if len(state.FollowUps) != 2 || state.FollowUps[0].ID != first.ID {
		t.Fatalf("partial clear durable: %+v", state)
	}
}

type rejectCancellationTerminalJournal struct{ storage.Journal }

func (j rejectCancellationTerminalJournal) Append(ctx context.Context, commit storage.Commit) (domain.EventSeq, error) {
	for _, event := range commit.Events {
		if event.Type.Terminal() {
			return 0, errors.New("terminal unavailable")
		}
	}
	return j.Journal.Append(ctx, commit)
}
func TestRealParkedCancellationRetriesAfterInteractionSettlement(t *testing.T) {
	for _, interaction := range []string{"question", "approval"} {
		for _, failure := range []string{"replay", "terminal"} {
			t.Run(interaction+"/"+failure, func(t *testing.T) {
				var svc *Service
				var backend *sqlite.Backend
				if interaction == "question" {
					svc, backend, _ = newQuestionService(t)
				} else {
					svc, backend, _ = newApprovalService(t, 5*time.Minute)
				}
				ctx := context.Background()
				sid := domain.SessionID("real-cancel-" + interaction + "-" + failure)
				mustCreateSession(t, backend, sid)
				rid, err := svc.Run(ctx, sid, "suspend for input")
				if err != nil {
					t.Fatal(err)
				}
				var question domain.Question
				var approval domain.Approval
				if interaction == "question" {
					question = waitForPendingQuestion(t, backend, rid)
				} else {
					approval = waitForPendingApproval(t, backend, rid)
				}
				waitFor(t, "pending registration", func() bool { svc.mu.Lock(); defer svc.mu.Unlock(); _, ok := svc.pending[rid]; return ok })
				if failure == "replay" {
					svc.dropSessionQueue(sid)
					svc.deps.Journal = &interruptedQueueReplay{Journal: backend}
				} else {
					svc.deps.Journal = rejectCancellationTerminalJournal{Journal: backend}
				}
				if !svc.Cancel(rid) {
					t.Fatal("initial cancel unavailable")
				}
				if interaction == "question" {
					row, err := backend.GetQuestion(ctx, question.ID)
					if err != nil || row.Status != domain.QuestionCancelled {
						t.Fatalf("question not cancelled: %+v %v", row, err)
					}
				} else {
					row, err := backend.GetApproval(ctx, approval.ID)
					if err != nil || row.Decision != domain.ApprovalCancelled {
						t.Fatalf("approval not cancelled: %+v %v", row, err)
					}
				}
				run, err := backend.GetRun(ctx, rid)
				if err != nil || run.Status.Terminal() {
					t.Fatalf("failed terminal committed: %+v %v", run, err)
				}
				svc.deps.Journal = backend
				if !svc.Cancel(rid) {
					t.Fatal("retry unavailable")
				}
				run, err = backend.GetRun(ctx, rid)
				if err != nil || run.Status != domain.RunCancelled {
					t.Fatalf("retry did not cancel real parked run: %+v %v", run, err)
				}
				events := replayAll(t, backend, rid)
				if countTerminal(events) != 1 {
					t.Fatalf("terminal count = %d", countTerminal(events))
				}
				cancelType := domain.EventUserQuestionCancelled
				if interaction == "approval" {
					cancelType = domain.EventToolApprovalCancelled
				}
				count := 0
				for _, event := range events {
					if event.Type == cancelType {
						count++
					}
				}
				if count != 1 {
					t.Fatalf("interaction cancelled events = %d", count)
				}
			})
		}
	}
}
func TestParkedCancellationPreservesInteractionDecisionWinner(t *testing.T) {
	for _, interaction := range []string{"question", "approval"} {
		t.Run(interaction, func(t *testing.T) {
			var svc *Service
			var backend *sqlite.Backend
			if interaction == "question" {
				svc, backend, _ = newQuestionService(t)
			} else {
				svc, backend, _ = newApprovalService(t, 5*time.Minute)
			}
			ctx := context.Background()
			sid := domain.SessionID("decision-wins-" + interaction)
			mustCreateSession(t, backend, sid)
			rid, err := svc.Run(ctx, sid, "suspend for input")
			if err != nil {
				t.Fatal(err)
			}
			if interaction == "question" {
				question := waitForPendingQuestion(t, backend, rid)
				if _, err := backend.AnswerQuestion(ctx, question.ID, "blue"); err != nil {
					t.Fatal(err)
				}
			} else {
				approval := waitForPendingApproval(t, backend, rid)
				if _, err := backend.DecideApproval(ctx, approval.ID, domain.ApprovalApproved); err != nil {
					t.Fatal(err)
				}
			}
			waitFor(t, "pending registration", func() bool { svc.mu.Lock(); defer svc.mu.Unlock(); _, ok := svc.pending[rid]; return ok })
			if !svc.Cancel(rid) {
				t.Fatal("cancel unavailable")
			}
			run, err := backend.GetRun(ctx, rid)
			if err != nil || run.Status.Terminal() {
				t.Fatalf("cancellation stole settled decision: %+v %v", run, err)
			}
		})
	}
}

func TestQueueDequeueExpectedIDRetainsNewerTurn(t *testing.T) {
	svc, backend := newQueueTestService(t, testsupport.NewEchoModel())
	ctx := context.Background()
	mustCreateSession(t, backend, "dequeue-cas")
	if err := backend.CreateRun(ctx, domain.Run{ID: "dequeue-carrier", SessionID: "dequeue-cas", Status: domain.RunActive}); err != nil {
		t.Fatal(err)
	}
	svc.active["dequeue-carrier"] = func() {}
	svc.runSessions["dequeue-carrier"] = "dequeue-cas"
	inspected, err := svc.FollowUp(ctx, "dequeue-cas", "inspected")
	if err != nil {
		t.Fatal(err)
	}
	newest, err := svc.FollowUp(ctx, "dequeue-cas", "newer unseen options")
	if err != nil {
		t.Fatal(err)
	}
	if item, removed, err := svc.Dequeue(ctx, "dequeue-cas", inspected.ID); err != nil || removed {
		t.Fatalf("stale inspection removed turn: %+v %v %v", item, removed, err)
	}
	if state := mustQueueState(t, svc, ctx, "dequeue-cas", ""); len(state.FollowUps) != 2 {
		t.Fatalf("CAS mutated live queue: %+v", state)
	}
	svc.dropSessionQueue("dequeue-cas")
	if state := mustQueueState(t, svc, ctx, "dequeue-cas", ""); len(state.FollowUps) != 2 {
		t.Fatalf("CAS mutated durable queue: %+v", state)
	}
	if item, removed, err := svc.Dequeue(ctx, "dequeue-cas", newest.ID); err != nil || !removed || item.ID != newest.ID {
		t.Fatalf("matched dequeue failed: %+v %v %v", item, removed, err)
	}
}

func TestFailedAdmissionQueueControlsRemainDurableAfterSeal(t *testing.T) {
	model := newGateModel()
	svc, backend := newQueueTestService(t, model)
	ctx := context.Background()
	mustCreateSession(t, backend, "sealed-controls")
	failed := make(chan error, 1)
	svc.deps.PrimaryRuns = queueFailingPrimaryStore{PrimaryRunStore: backend, failed: failed}
	rid, err := svc.Run(ctx, "sealed-controls", "initial")
	if err != nil {
		t.Fatal(err)
	}
	<-model.entered
	var queued []domain.QueuedTurn
	for _, text := range []string{"remove", "clear", "dequeue"} {
		item, err := svc.FollowUpWithOptions(ctx, "sealed-controls", domain.QueuedTurn{Text: text, Thinking: domain.ThinkingLevelHigh, Attachments: []domain.Attachment{{Name: "captured.png", MimeType: "image/png", Data: []byte{1, 2, 3}}}})
		if err != nil {
			t.Fatal(err)
		}
		queued = append(queued, item)
	}
	close(model.release)
	if err := <-failed; err == nil {
		t.Fatal("admission failure missing")
	}
	svc.WaitIdle(ctx)
	svc.deps.PrimaryRuns = backend
	if _, ok, err := svc.QueueRemove(ctx, "sealed-controls", queued[0].ID); err != nil || !ok {
		t.Fatalf("sealed remove = %v %v", ok, err)
	}
	svc.dropSessionQueue("sealed-controls")
	state := mustQueueState(t, svc, ctx, "sealed-controls", "")
	if len(state.FollowUps) != 2 || !reflect.DeepEqual(state.FollowUps[1].Attachments, queued[2].Attachments) {
		t.Fatalf("remove replay lost remaining DTOs: %+v", state)
	}
	if turn, ok, err := svc.Dequeue(ctx, "sealed-controls", queued[2].ID); err != nil || !ok || !reflect.DeepEqual(turn.Attachments, queued[2].Attachments) {
		t.Fatalf("sealed dequeue = %+v %v %v", turn, ok, err)
	}
	svc.dropSessionQueue("sealed-controls")
	if state := mustQueueState(t, svc, ctx, "sealed-controls", ""); len(state.FollowUps) != 1 {
		t.Fatalf("dequeue replay = %+v", state)
	}
	if state, err := svc.ClearQueue(ctx, "sealed-controls"); err != nil || len(state.FollowUps) != 1 {
		t.Fatalf("sealed clear = %+v %v", state, err)
	}
	svc.dropSessionQueue("sealed-controls")
	if state := mustQueueState(t, svc, ctx, "sealed-controls", ""); len(state.FollowUps) != 0 {
		t.Fatalf("clear replay = %+v", state)
	}
	events := journalEvents(t, backend, rid)
	if !events[len(events)-1].Type.Terminal() {
		t.Fatal("control appended after real terminal")
	}
	runs, err := backend.ListRunsBySession(ctx, "sealed-controls")
	if err != nil || len(runs) != 1 {
		t.Fatalf("queue controls created model run: %+v %v", runs, err)
	}
}
func TestQueuePayloadLimitRejectsFullDTOBeforeAdmission(t *testing.T) {
	svc, backend := newQueueTestService(t, testsupport.NewEchoModel())
	ctx := context.Background()
	mustCreateSession(t, backend, "payload-boundary")
	if err := backend.CreateRun(ctx, domain.Run{ID: "payload-carrier", SessionID: "payload-boundary", Status: domain.RunActive}); err != nil {
		t.Fatal(err)
	}
	svc.active["payload-carrier"] = func() {}
	svc.runSessions["payload-carrier"] = "payload-boundary"
	_, err := svc.FollowUpWithOptions(ctx, "payload-boundary", domain.QueuedTurn{Text: "full image", Attachments: []domain.Attachment{{Name: "large.png", MimeType: "image/png", Data: make([]byte, svc.MaxEventPayloadBytes())}}})
	if !errors.Is(err, ErrQueuePayloadTooLarge) {
		t.Fatalf("oversized DTO error = %v", err)
	}
	if state := mustQueueState(t, svc, ctx, "payload-boundary", ""); len(state.FollowUps) != 0 {
		t.Fatal("oversized DTO visible")
	}
	svc.dropSessionQueue("payload-boundary")
	if state := mustQueueState(t, svc, ctx, "payload-boundary", ""); len(state.FollowUps) != 0 {
		t.Fatal("oversized DTO durable")
	}

	accepted, err := svc.FollowUpWithOptions(ctx, "payload-boundary", domain.QueuedTurn{Text: "bounded image", Attachments: []domain.Attachment{{Name: "bounded.png", MimeType: "image/png", Data: make([]byte, 20_000)}}})
	if err != nil {
		t.Fatal(err)
	}
	if restored, ok, err := svc.Dequeue(ctx, "payload-boundary", accepted.ID); err != nil || !ok || !reflect.DeepEqual(restored.Attachments, accepted.Attachments) {
		t.Fatalf("bounded DTO did not restore intact: %v %v", ok, err)
	}
	for _, event := range journalEvents(t, backend, "payload-carrier") {
		if len(event.Payload) > svc.MaxEventPayloadBytes() {
			t.Fatalf("event exceeds encoded bound: %d", len(event.Payload))
		}
	}
}

type failingQueueControlReplay struct {
	storage.Journal
	once         bool
	closeFailure bool
}

func (j *failingQueueControlReplay) Replay(ctx context.Context, rid domain.RunID, after domain.EventSeq) (storage.Iterator[storage.Entry], error) {
	it, err := j.Journal.Replay(ctx, rid, after)
	if err != nil {
		return nil, err
	}
	if strings.HasPrefix(string(rid), "sessctl_queue_") && !j.once {
		j.once = true
		if j.closeFailure {
			return queueControlCloseFailure{Iterator: it}, nil
		}
		return &queueReplayFailure{Iterator: it}, nil
	}
	return it, nil
}

type queueControlCloseFailure struct {
	storage.Iterator[storage.Entry]
}

func (it queueControlCloseFailure) Close() error {
	return errors.Join(it.Iterator.Close(), errors.New("control replay close failed"))
}
func TestQueueControlReplayFailureNeverPublishesPartialState(t *testing.T) {
	for _, closeFailure := range []bool{false, true} {
		t.Run(fmt.Sprint(closeFailure), func(t *testing.T) {
			svc, backend := newQueueTestService(t, testsupport.NewEchoModel())
			ctx := context.Background()
			mustCreateSession(t, backend, "controls-retry")
			if err := backend.CreateRun(ctx, domain.Run{ID: "control-carrier", SessionID: "controls-retry", Status: domain.RunCompleted, CreatedAt: 1}); err != nil {
				t.Fatal(err)
			}
			mapper := newEventMapper("control-carrier", svc.MaxEventPayloadBytes())
			if _, err := backend.Append(ctx, storage.Commit{RunID: "control-carrier", Events: []domain.RunEvent{mapper.build(domain.EventTurnQueued, payloadTurnQueued{QueueID: "removed", Text: "original", Track: domain.QueueTrackFollowUp}), mapper.build(domain.EventRunCompleted, payloadRunCompleted{})}}); err != nil {
				t.Fatal(err)
			}
			control := queueControlRunID("control-carrier")
			if _, err := backend.Append(ctx, storage.Commit{RunID: control, Events: []domain.RunEvent{newEventMapper(control, svc.MaxEventPayloadBytes()).build(domain.EventTurnDequeued, payloadTurnDequeued{QueueID: "removed"})}}); err != nil {
				t.Fatal(err)
			}
			svc.deps.Journal = &failingQueueControlReplay{Journal: backend, closeFailure: closeFailure}
			if state, err := svc.QueueState(ctx, "controls-retry", ""); err == nil {
				t.Fatalf("control failure hidden: %+v", state)
			}
			if state := mustQueueState(t, svc, ctx, "controls-retry", ""); len(state.FollowUps) != 0 {
				t.Fatalf("control retry lost removal: %+v", state)
			}
			// An old carrier's controls cannot remove a re-admitted same-ID turn.
			if err := backend.CreateRun(ctx, domain.Run{ID: "new-carrier", SessionID: "controls-retry", Status: domain.RunActive, CreatedAt: 2}); err != nil {
				t.Fatal(err)
			}
			if _, err := backend.Append(ctx, storage.Commit{RunID: "new-carrier", Events: []domain.RunEvent{newEventMapper("new-carrier", svc.MaxEventPayloadBytes()).build(domain.EventTurnQueued, payloadTurnQueued{QueueID: "removed", Text: "readmitted", Track: domain.QueueTrackFollowUp})}}); err != nil {
				t.Fatal(err)
			}
			svc.dropSessionQueue("controls-retry")
			if state := mustQueueState(t, svc, ctx, "controls-retry", ""); len(state.FollowUps) != 1 {
				t.Fatalf("old controls applied to new carrier: %+v", state)
			}
		})
	}
}

type failQueueControlAppend struct{ storage.Journal }

func (j failQueueControlAppend) Append(ctx context.Context, commit storage.Commit) (domain.EventSeq, error) {
	if strings.HasPrefix(string(commit.RunID), "sessctl_queue_") {
		return 0, errors.New("control write unavailable")
	}
	return j.Journal.Append(ctx, commit)
}
func TestSealedClearControlWriteFailureRetainsEntireQueue(t *testing.T) {
	svc, backend := newQueueTestService(t, testsupport.NewEchoModel())
	ctx := context.Background()
	mustCreateSession(t, backend, "control-write-fail")
	if err := backend.CreateRun(ctx, domain.Run{ID: "sealed-clear", SessionID: "control-write-fail", Status: domain.RunCompleted}); err != nil {
		t.Fatal(err)
	}
	mapper := newEventMapper("sealed-clear", svc.MaxEventPayloadBytes())
	if _, err := backend.Append(ctx, storage.Commit{RunID: "sealed-clear", Events: []domain.RunEvent{mapper.build(domain.EventTurnQueued, payloadTurnQueued{QueueID: "one", Text: "one", Track: domain.QueueTrackFollowUp}), mapper.build(domain.EventTurnQueued, payloadTurnQueued{QueueID: "two", Text: "two", Track: domain.QueueTrackFollowUp}), mapper.build(domain.EventRunCompleted, payloadRunCompleted{})}}); err != nil {
		t.Fatal(err)
	}
	svc.deps.Journal = failQueueControlAppend{Journal: backend}
	if _, err := svc.ClearQueue(ctx, "control-write-fail"); err == nil {
		t.Fatal("control failure hidden")
	}
	if state := mustQueueState(t, svc, ctx, "control-write-fail", ""); len(state.FollowUps) != 2 {
		t.Fatal("failed control mutated live queue")
	}
	svc.dropSessionQueue("control-write-fail")
	if state := mustQueueState(t, svc, ctx, "control-write-fail", ""); len(state.FollowUps) != 2 {
		t.Fatal("failed control mutated durable queue")
	}
	svc.deps.Journal = backend
	if state, err := svc.ClearQueue(ctx, "control-write-fail"); err != nil || len(state.FollowUps) != 2 {
		t.Fatalf("clear retry = %+v %v", state, err)
	}
	svc.dropSessionQueue("control-write-fail")
	if state := mustQueueState(t, svc, ctx, "control-write-fail", ""); len(state.FollowUps) != 0 {
		t.Fatal("clear retry not durable")
	}
}
