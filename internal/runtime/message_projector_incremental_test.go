package runtime

import (
	"context"
	"errors"
	"reflect"
	"sync"
	"testing"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/storage"
	"agent-vivy/internal/storage/sqlite"
	"agent-vivy/internal/testsupport"
)

type recordingJournal struct {
	storage.Journal
	mu     sync.Mutex
	afters []domain.EventSeq
}

func (j *recordingJournal) Replay(ctx context.Context, runID domain.RunID, after domain.EventSeq) (storage.Iterator[storage.Entry], error) {
	j.mu.Lock()
	j.afters = append(j.afters, after)
	j.mu.Unlock()
	return j.Journal.Replay(ctx, runID, after)
}

func (j *recordingJournal) recorded() []domain.EventSeq {
	j.mu.Lock()
	defer j.mu.Unlock()
	return append([]domain.EventSeq(nil), j.afters...)
}

type countingMessages struct {
	storage.MessageStore
	mu    sync.Mutex
	lists int
}

func (m *countingMessages) ListMessages(ctx context.Context, sessionID domain.SessionID) ([]domain.Message, error) {
	m.mu.Lock()
	m.lists++
	m.mu.Unlock()
	return m.MessageStore.ListMessages(ctx, sessionID)
}

func (m *countingMessages) listCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.lists
}

func projectorCursorFixture(t *testing.T, sessionID domain.SessionID, runID domain.RunID) (*Service, *sqlite.Backend) {
	t.Helper()
	svc, backend, _ := newTestService(t, testsupport.NewEchoModel())
	ctx := context.Background()
	if err := backend.CreateSession(ctx, domain.Session{ID: sessionID, Title: "cursor", CreatedAt: 1}); err != nil {
		t.Fatalf("create session: %v", err)
	}
	if err := backend.CreateRun(ctx, domain.Run{ID: runID, SessionID: sessionID, Status: domain.RunActive, CreatedAt: 1}); err != nil {
		t.Fatalf("create run: %v", err)
	}
	return svc, backend
}

func TestProjectorLiveCursorSkipsEarlierJournalEvents(t *testing.T) {
	ctx := context.Background()
	sessionID := domain.SessionID("sess-projector-cursor")
	runID := domain.RunID("run-projector-cursor")
	svc, backend := projectorCursorFixture(t, sessionID, runID)

	journal := &recordingJournal{Journal: backend}
	messages := &countingMessages{MessageStore: backend}
	svc.deps.Journal = journal
	svc.deps.Messages = messages

	trajCommit(t, ctx, backend, runID,
		trajEvent(domain.EventRunStarted, 10, payloadRunStarted{Provider: "test", Model: "test-model", Mode: "chat"}),
		trajEvent(domain.EventModelRequest, 20, payloadModelRequest{}),
		trajEvent(domain.EventModelDelta, 30, payloadModelDelta{Delta: "before tool"}),
		trajEvent(domain.EventToolRequested, 40, payloadToolRequested{ToolCallID: "call-1", ToolName: "echo_info", Args: map[string]any{"text": "hi"}}),
		trajEvent(domain.EventToolFinished, 50, payloadToolFinished{ToolCallID: "call-1", ToolName: "echo_info", Result: "tool result"}),
	)
	if err := svc.projectRunMessages(ctx, sessionID, runID); err != nil {
		t.Fatalf("first projection: %v", err)
	}
	if got := journal.recorded(); !reflect.DeepEqual(got, []domain.EventSeq{0}) {
		t.Fatalf("first replay offsets = %v, want a full replay from 0", got)
	}
	if messages.listCount() != 1 {
		t.Fatalf("first projection listed messages %d times, want the one-time adoption scan", messages.listCount())
	}

	trajCommit(t, ctx, backend, runID,
		trajEvent(domain.EventModelRequest, 60, payloadModelRequest{}),
		trajEvent(domain.EventModelDelta, 70, payloadModelDelta{Delta: "final answer"}),
		trajEventVersion(domain.EventModelCompleted, 80, v2Completion("final answer"), 2),
	)
	if err := svc.projectRunMessages(ctx, sessionID, runID); err != nil {
		t.Fatalf("incremental projection: %v", err)
	}
	afters := journal.recorded()
	if len(afters) != 2 || afters[1] != 5 {
		t.Fatalf("replay offsets = %v, want the second fold to resume after the first exchange", afters)
	}
	if messages.listCount() != 1 {
		t.Fatalf("incremental projection listed messages again (%d lists)", messages.listCount())
	}

	got, err := backend.ListMessages(ctx, sessionID)
	if err != nil {
		t.Fatalf("list projection: %v", err)
	}
	if len(got) != 4 {
		t.Fatalf("projected rows = %+v, want four", got)
	}
	if got[0].Content != "before tool" || got[1].ToolCallID != "call-1" ||
		got[2].Content != "tool result" || got[3].Content != "final answer" {
		t.Fatalf("incremental projection = %+v", got)
	}

	// A no-op fold still advances to the run head and never duplicates rows.
	if err := svc.projectRunMessages(ctx, sessionID, runID); err != nil {
		t.Fatalf("idle projection: %v", err)
	}
	if afters := journal.recorded(); len(afters) != 3 || afters[2] != 8 {
		t.Fatalf("idle replay offsets = %v, want one fold from the run head", afters)
	}
	again, err := backend.ListMessages(ctx, sessionID)
	if err != nil || !reflect.DeepEqual(again, got) {
		t.Fatalf("idle projection changed rows: %+v %v", again, err)
	}
}

func TestProjectorLiveCursorCarriesPendingDeltas(t *testing.T) {
	ctx := context.Background()
	sessionID := domain.SessionID("sess-projector-cursor-pending")
	runID := domain.RunID("run-projector-cursor-pending")
	svc, backend := projectorCursorFixture(t, sessionID, runID)

	journal := &recordingJournal{Journal: backend}
	svc.deps.Journal = journal

	trajCommit(t, ctx, backend, runID,
		trajEvent(domain.EventRunStarted, 10, payloadRunStarted{Provider: "test", Model: "test-model", Mode: "chat"}),
		trajEvent(domain.EventModelRequest, 20, payloadModelRequest{}),
		trajEvent(domain.EventModelDelta, 30, payloadModelDelta{Delta: "first half "}),
	)
	if err := svc.projectRunMessages(ctx, sessionID, runID); err != nil {
		t.Fatalf("partial projection: %v", err)
	}
	svc.projectionMu.Lock()
	state, tracked := svc.projectionStateLocked(runID)
	svc.projectionMu.Unlock()
	if !tracked || state.pending != "first half " || state.seq != 3 {
		t.Fatalf("cursor after partial output = %+v/%v", state, tracked)
	}

	content := "first half and second half"
	trajCommit(t, ctx, backend, runID,
		trajEvent(domain.EventModelDelta, 40, payloadModelDelta{Delta: "and second half"}),
		trajEventVersion(domain.EventModelCompleted, 50, v2Completion(content), 2),
	)
	if err := svc.projectRunMessages(ctx, sessionID, runID); err != nil {
		t.Fatalf("completed projection: %v", err)
	}
	svc.projectionMu.Lock()
	state, tracked = svc.projectionStateLocked(runID)
	svc.projectionMu.Unlock()
	if !tracked || state.pending != "" || state.seq != 5 {
		t.Fatalf("cursor after completion = %+v/%v", state, tracked)
	}
	got, err := backend.ListMessages(ctx, sessionID)
	if err != nil {
		t.Fatalf("list projection: %v", err)
	}
	if len(got) != 1 || got[0].Content != content {
		t.Fatalf("carried deltas projected %+v", got)
	}
	if afters := journal.recorded(); len(afters) != 2 || afters[1] != 3 {
		t.Fatalf("replay offsets = %v, want the second fold to resume after seq 30", afters)
	}
}

func TestProjectorLiveCursorDoesNotAdvanceOnConflict(t *testing.T) {
	ctx := context.Background()
	sessionID := domain.SessionID("sess-projector-cursor-conflict")
	runID := domain.RunID("run-projector-cursor-conflict")
	svc, backend := projectorCursorFixture(t, sessionID, runID)

	trajCommit(t, ctx, backend, runID,
		trajEvent(domain.EventModelDelta, 10, payloadModelDelta{Delta: "before tool"}),
		trajEvent(domain.EventToolRequested, 20, payloadToolRequested{ToolCallID: "call-1", ToolName: "echo_info", Args: map[string]any{"text": "hi"}}),
		trajEvent(domain.EventToolFinished, 30, payloadToolFinished{ToolCallID: "call-1", ToolName: "echo_info", Result: "tool result"}),
	)
	if err := svc.projectRunMessages(ctx, sessionID, runID); err != nil {
		t.Fatalf("seed projection: %v", err)
	}

	content := "durable answer"
	trajCommit(t, ctx, backend, runID,
		trajEvent(domain.EventModelRequest, 40, payloadModelRequest{}),
		trajEvent(domain.EventModelDelta, 50, payloadModelDelta{Delta: content}),
		trajEventVersion(domain.EventModelCompleted, 60, v2Completion(content), 2),
	)
	conflicting := domain.Message{
		ID: projectedMessageID(runID, 6, "0"), SessionID: sessionID, RunID: runID,
		Role: domain.RoleAssistant, CreatedAt: 60, Content: "someone else wrote this",
	}
	if err := backend.AppendMessage(ctx, conflicting); err != nil {
		t.Fatalf("seed conflicting row: %v", err)
	}

	for attempt := 0; attempt < 2; attempt++ {
		err := svc.projectRunMessages(ctx, sessionID, runID)
		if !errors.Is(err, storage.ErrProjectionConflict) {
			t.Fatalf("attempt %d error = %v, want projection conflict", attempt+1, err)
		}
		svc.projectionMu.Lock()
		state, tracked := svc.projectionStateLocked(runID)
		svc.projectionMu.Unlock()
		if !tracked || state.seq != 3 {
			t.Fatalf("attempt %d advanced the cursor to %+v/%v", attempt+1, state, tracked)
		}
	}
}

func TestProjectorLiveCursorSuppressesNoContextRuns(t *testing.T) {
	ctx := context.Background()
	sessionID := domain.SessionID("sess-projector-cursor-nocontext")
	runID := domain.RunID("run-projector-cursor-nocontext")
	svc, backend := projectorCursorFixture(t, sessionID, runID)
	journal := &recordingJournal{Journal: backend}
	messages := &countingMessages{MessageStore: backend}
	svc.deps.Journal = journal
	svc.deps.Messages = messages

	trajCommit(t, ctx, backend, runID,
		trajEvent(domain.EventRunStarted, 10, payloadRunStarted{Provider: "test", Model: "test-model", Mode: "shell", NoContext: true}),
		trajEvent(domain.EventModelDelta, 20, payloadModelDelta{Delta: "shell output"}),
		trajEventVersion(domain.EventModelCompleted, 30, v2Completion("shell output"), 2),
	)
	if err := svc.projectRunMessages(ctx, sessionID, runID); err != nil {
		t.Fatalf("no-context projection: %v", err)
	}
	if messages.listCount() != 0 {
		t.Fatalf("no-context projection listed messages %d times", messages.listCount())
	}
	svc.projectionMu.Lock()
	state, tracked := svc.projectionStateLocked(runID)
	svc.projectionMu.Unlock()
	if !tracked || !state.noContext {
		t.Fatalf("no-context cursor = %+v/%v, want a suppressing marker", state, tracked)
	}
	if err := svc.projectRunMessages(ctx, sessionID, runID); err != nil {
		t.Fatalf("repeat no-context projection: %v", err)
	}
	if afters := journal.recorded(); len(afters) != 1 {
		t.Fatalf("no-context cursor replayed again: %v", afters)
	}
	got, err := backend.ListMessages(ctx, sessionID)
	if err != nil || len(got) != 0 {
		t.Fatalf("no-context projection rows = %+v %v", got, err)
	}
}

func TestProjectorReconcileSeedsLiveCursorAndDropsTerminal(t *testing.T) {
	ctx := context.Background()
	sessionID := domain.SessionID("sess-projector-cursor-reconcile")
	runID := domain.RunID("run-projector-cursor-reconcile")
	svc, backend := projectorCursorFixture(t, sessionID, runID)

	content := "reconciled answer"
	trajCommit(t, ctx, backend, runID,
		trajEvent(domain.EventModelRequest, 10, payloadModelRequest{}),
		trajEvent(domain.EventModelDelta, 20, payloadModelDelta{Delta: content}),
		trajEventVersion(domain.EventModelCompleted, 30, v2Completion(content), 2),
	)
	if err := svc.ReconcileSessionMessages(ctx, sessionID); err != nil {
		t.Fatalf("first reconcile: %v", err)
	}
	svc.projectionMu.Lock()
	state, tracked := svc.projectionStateLocked(runID)
	svc.projectionMu.Unlock()
	if !tracked || state.seq != 3 {
		t.Fatalf("reconcile cursor = %+v/%v, want the live run head", state, tracked)
	}

	journal := &recordingJournal{Journal: backend}
	svc.deps.Journal = journal
	if err := svc.ReconcileSessionMessages(ctx, sessionID); err != nil {
		t.Fatalf("second reconcile: %v", err)
	}
	if afters := journal.recorded(); len(afters) != 1 || afters[0] != 3 {
		t.Fatalf("reconcile replay offsets = %v, want a resume from the seeded cursor", afters)
	}

	if err := backend.SetRunStatus(ctx, runID, domain.RunCompleted); err != nil {
		t.Fatalf("complete run: %v", err)
	}
	if err := svc.ReconcileSessionMessages(ctx, sessionID); err != nil {
		t.Fatalf("terminal reconcile: %v", err)
	}
	svc.projectionMu.Lock()
	_, tracked = svc.projectionStateLocked(runID)
	svc.projectionMu.Unlock()
	if tracked {
		t.Fatal("terminal run kept a projection cursor")
	}
}
