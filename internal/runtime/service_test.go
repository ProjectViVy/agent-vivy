package runtime

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/provider"
	"agent-vivy/internal/storage"
	"agent-vivy/internal/storage/sqlite"
	"agent-vivy/internal/testsupport"
	"agent-vivy/internal/tools"
)

func newTestService(t *testing.T, model domain.ChatModel) (*Service, *sqlite.Backend, *testSink) {
	t.Helper()
	ctx := context.Background()

	backend, err := sqlite.Open(ctx, filepath.Join(t.TempDir(), "journal.db"))
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { _ = backend.Close() })

	ts, err := tools.Builtin(backend).Resolve([]string{tools.EchoInfoName})
	if err != nil {
		t.Fatalf("resolve tools: %v", err)
	}
	eng, err := NewEngine(ctx, WrapModel(model), ts, EngineConfig{StreamBuffer: 8, MaxEventPayloadBytes: 64 << 10})
	if err != nil {
		t.Fatalf("new engine: %v", err)
	}
	sink := newTestSink()
	svc := NewService(eng, "test", "test-model", ServiceDeps{
		Journal: backend, Runs: backend, Messages: backend, Sessions: backend, Sink: sink, Truncations: backend,
	})
	return svc, backend, sink
}

// mustCreateSession establishes the durable session row that message
// storage requires (fail-closed position allocation). Test fixtures that
// run or append messages for a session must call this first.
func mustCreateSession(t *testing.T, sessions storage.SessionStore, id domain.SessionID) {
	t.Helper()
	if err := sessions.CreateSession(context.Background(), domain.Session{ID: id, Title: "fixture", CreatedAt: 1}); err != nil {
		t.Fatalf("create session %s: %v", id, err)
	}
}

type blockingWorkspaceAllocator struct {
	entered chan struct{}
	release chan struct{}
	path    string
}

func (allocator *blockingWorkspaceAllocator) Ensure(_ context.Context, runID domain.RunID) (Workspace, error) {
	close(allocator.entered)
	<-allocator.release
	return Workspace{ID: string(runID), Path: allocator.path}, nil
}

func TestSetSessionWorkspaceSerializesWithFirstRunAllocation(t *testing.T) {
	svc, backend, _ := newTestService(t, testsupport.NewEchoModel())
	ctx := context.Background()
	sessionID := domain.SessionID("session-workspace-race")
	if err := backend.CreateSession(ctx, domain.Session{ID: sessionID, Title: "race", CreatedAt: 1}); err != nil {
		t.Fatal(err)
	}
	allocator := &blockingWorkspaceAllocator{entered: make(chan struct{}), release: make(chan struct{}), path: t.TempDir()}
	svc.deps.Workspaces = allocator
	t.Cleanup(func() { svc.CancelAll(); svc.WaitIdle(context.Background()) })

	runDone := make(chan error, 1)
	go func() { _, err := svc.Run(ctx, sessionID, "start"); runDone <- err }()
	<-allocator.entered
	setDone := make(chan error, 1)
	go func() { _, err := svc.SetSessionWorkspace(ctx, sessionID, t.TempDir()); setDone <- err }()
	select {
	case err := <-setDone:
		t.Fatalf("workspace mutation escaped the startup fence: %v", err)
	case <-time.After(25 * time.Millisecond):
	}
	close(allocator.release)
	if err := <-runDone; err != nil {
		t.Fatalf("start run: %v", err)
	}
	if err := <-setDone; !errors.Is(err, storage.ErrConflict) {
		t.Fatalf("workspace mutation after first run = %v, want conflict", err)
	}
}

func TestChangeModelWhenIdleCommitsUnderRunStartupFence(t *testing.T) {
	svc := NewService(nil, "deepseek", "old-model", ServiceDeps{})
	persisted := false
	if err := svc.ChangeModelWhenIdle("anthropic", "new-model", func() error {
		persisted = true
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if !persisted {
		t.Fatal("model persistence callback was not called")
	}
	if providerName, modelID := svc.CurrentModel(); providerName != "anthropic" || modelID != "new-model" {
		t.Fatalf("current model = %q/%q", providerName, modelID)
	}

	sentinel := errors.New("save failed")
	if err := svc.ChangeModelWhenIdle("deepseek", "broken", func() error { return sentinel }); !errors.Is(err, sentinel) {
		t.Fatalf("persist failure = %v", err)
	}
	if providerName, modelID := svc.CurrentModel(); providerName != "anthropic" || modelID != "new-model" {
		t.Fatalf("failed persistence changed model = %q/%q", providerName, modelID)
	}

	svc.mu.Lock()
	svc.active["run-active"] = func() {}
	svc.mu.Unlock()
	called := false
	if err := svc.ChangeModelWhenIdle("deepseek", "later", func() error { called = true; return nil }); !errors.Is(err, ErrModelChangeBusy) {
		t.Fatalf("active run model change = %v", err)
	}
	if called {
		t.Fatal("busy model change called persistence")
	}
	svc.mu.Lock()
	delete(svc.active, "run-active")
	svc.snapshots["child-active"] = domain.PolicySnapshot{Profile: domain.PolicyProfileDefault, Hash: "hash"}
	svc.mu.Unlock()
	if err := svc.ChangeModelWhenIdle("deepseek", "later", func() error { return nil }); !errors.Is(err, ErrModelChangeBusy) {
		t.Fatalf("child worker model change = %v", err)
	}
	svc.cleanupRunState("child-active")
	if err := svc.ChangeModelWhenIdle("deepseek", "after-cleanup", func() error { return nil }); err != nil {
		t.Fatalf("cleanup left model selection permanently busy: %v", err)
	}
}

// testSink collects published events. The bus never delivers terminal
// events (it closes its subscribers instead), so the snapshot must stay
// terminal-free; the journal remains the place to assert the close.
type testSink struct {
	mu     sync.Mutex
	events []domain.RunEvent
}

func newTestSink() *testSink { return &testSink{} }

func (s *testSink) Publish(ev domain.RunEvent) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.events = append(s.events, ev)
}

func (s *testSink) snapshot() []domain.RunEvent {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]domain.RunEvent, len(s.events))
	copy(out, s.events)
	return out
}

func countTerminal(events []domain.RunEvent) int {
	n := 0
	for _, ev := range events {
		if ev.Type.Terminal() {
			n++
		}
	}
	return n
}

// waitForRunStatus polls the run row until it reaches want (bounded).
func waitForRunStatus(t *testing.T, runs storage.RunStore, runID domain.RunID, want domain.RunStatus) {
	t.Helper()
	deadline := time.Now().Add(30 * time.Second)
	var last domain.RunStatus
	for time.Now().Before(deadline) {
		r, err := runs.GetRun(context.Background(), runID)
		if err == nil {
			last = r.Status
			if r.Status == want {
				return
			}
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("run %s never reached status %s (last status %s)", runID, want, last)
}

// replayAll drains the journal for one run.
func replayAll(t *testing.T, j storage.Journal, runID domain.RunID) []domain.RunEvent {
	t.Helper()
	it, err := j.Replay(context.Background(), runID, 0)
	if err != nil {
		t.Fatalf("replay: %v", err)
	}
	defer it.Close()
	var out []domain.RunEvent
	for it.Next() {
		out = append(out, it.Value().Event)
	}
	if err := it.Err(); err != nil {
		t.Fatalf("replay iteration: %v", err)
	}
	return out
}

func TestServiceRunHappyPath(t *testing.T) {
	svc, backend, sink := newTestService(t, testsupport.NewEchoModel())
	mustCreateSession(t, backend, "sess-1")
	runID, err := svc.Run(context.Background(), "sess-1", "hello vivy")
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	waitForRunStatus(t, backend, runID, domain.RunCompleted)

	// The terminal publish lands right after the status flip; wait for it
	// so the snapshot is complete.
	deadline := time.Now().Add(5 * time.Second)
	for countTerminal(sink.snapshot()) == 0 {
		if time.Now().After(deadline) {
			t.Fatal("terminal event was never published to the sink")
		}
		time.Sleep(10 * time.Millisecond)
	}

	live := sink.snapshot()
	if len(live) == 0 || live[0].Type != domain.EventRunStarted {
		t.Fatalf("first published event = %+v, want run.started", live)
	}
	if n := countTerminal(live); n != 1 {
		t.Fatalf("terminal events published to the sink = %d, want 1", n)
	}

	// The journal holds the full sequence including the single terminal.
	events := replayAll(t, backend, runID)
	if len(events) < 4 {
		t.Fatalf("expected at least 4 events, got %d", len(events))
	}
	if events[0].Type != domain.EventRunStarted {
		t.Fatalf("first event = %s, want run.started", events[0].Type)
	}
	last := events[len(events)-1]
	if last.Type != domain.EventRunCompleted {
		t.Fatalf("last event = %s, want run.completed", last.Type)
	}
	if n := countTerminal(events); n != 1 {
		t.Fatalf("terminal events = %d, want exactly 1", n)
	}

	// Seq is monotonic 1..K with no gaps. Observed calls emit model.request
	// v3 and model.usage v2; model.completed and the terminal Observer
	// projection use v2; the remaining events retain their existing
	// versions when no Context View is selected.
	for i, ev := range events {
		if ev.Seq != domain.EventSeq(i+1) {
			t.Fatalf("event %d has seq %d, want %d", i, ev.Seq, i+1)
		}
		wantVersion := 1
		switch ev.Type {
		case domain.EventModelCompleted, domain.EventRunCompleted, domain.EventModelUsage:
			wantVersion = 2
		case domain.EventModelRequest:
			wantVersion = 3
		}
		if ev.PayloadVersion != wantVersion {
			t.Fatalf("event %d (%s) payload version = %d, want %d", i, ev.Type, ev.PayloadVersion, wantVersion)
		}
	}

	// Middle shape: observed request + delta* + the call's finish record,
	// then metadata-only model.completed before the terminal. Reassembly is
	// byte-for-byte and independently checked against the completion
	// digest/length.
	var deltas strings.Builder
	var completed payloadModelCompletedV2
	for _, ev := range events[1 : len(events)-1] {
		switch ev.Type {
		case domain.EventModelRequest:
			// Request digest is recorded before the model stream.
		case domain.EventModelDelta:
			deltas.WriteString(payloadDeltaOf(t, ev.Payload))
		case domain.EventModelCompleted:
			completed = payloadCompletedOf(t, ev.Payload)
		case domain.EventModelUsage, domain.EventModelCallFinished:
			// Observed-call lifecycle records.
		default:
			t.Fatalf("unexpected mid-run event %s", ev.Type)
		}
	}
	want := "test response to: hello vivy"
	if deltas.String() != want {
		t.Fatalf("reassembled deltas = %q, want %q", deltas.String(), want)
	}
	if completed.ContentSHA256 != sha256Hex([]byte(want)) || completed.ByteLen != len([]byte(want)) {
		t.Fatalf("model.completed metadata = %+v, want sha/bytes for %q", completed, want)
	}

	// The conversation log mirrors the user turn and the assistant reply.
	msgs, err := backend.ListMessages(context.Background(), "sess-1")
	if err != nil {
		t.Fatalf("list messages: %v", err)
	}
	if len(msgs) != 2 {
		t.Fatalf("messages = %d, want 2 (user + assistant)", len(msgs))
	}
	if msgs[0].Role != domain.RoleUser || msgs[0].Content != "hello vivy" || msgs[0].RunID != runID {
		t.Fatalf("user message = %+v", msgs[0])
	}
	if msgs[1].Role != domain.RoleAssistant || msgs[1].Content != want || msgs[1].RunID != runID {
		t.Fatalf("assistant message = %+v", msgs[1])
	}
}

// TestServiceRunBindsToolsOnKeywordlessRequest is the regression gate for
// the retired keyword tool selector: a keyword-less (here: Chinese) user
// message must still reach the model with a bound tool surface, not an
// empty selection. The v3 request reports the actual bound tools at the
// model boundary: echo_info is deferred behind official tool search, so
// the surface names the fixed-visible/search tools the model may call.
func TestServiceRunBindsToolsOnKeywordlessRequest(t *testing.T) {
	svc, backend, _ := newTestService(t, testsupport.NewEchoModel())
	ctx := context.Background()
	mustCreateSession(t, backend, "sess-zh-tools")

	runID, err := svc.Run(ctx, "sess-zh-tools", "你现在有什么工具？")
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	waitForRunStatus(t, backend, runID, domain.RunCompleted)

	var req payloadModelRequestV3
	found := false
	for _, ev := range replayAll(t, backend, runID) {
		if ev.Type == domain.EventModelRequest {
			mustUnmarshal(t, ev.Payload, &req)
			found = true
			break
		}
	}
	if !found {
		t.Fatal("run missing model.request")
	}
	if len(req.SelectedTools) == 0 {
		t.Fatal("keyword-less request bound no tools")
	}
	sawSearch := false
	for _, name := range req.SelectedTools {
		if name == officialToolSearchName {
			sawSearch = true
		}
	}
	if !sawSearch {
		t.Fatalf("bound tools %v missing %s (deferred tools surface through it)", req.SelectedTools, officialToolSearchName)
	}
}

// blockingModel blocks until ctx is done, then surfaces the cancellation.
type blockingModel struct{}

func (blockingModel) Stream(ctx context.Context, _ []*domain.Message) (domain.Stream[*domain.Message], error) {
	<-ctx.Done()
	return nil, ctx.Err()
}

func TestServiceRunCancelled(t *testing.T) {
	svc, backend, _ := newTestService(t, blockingModel{})
	mustCreateSession(t, backend, "sess-1")
	runID, err := svc.Run(context.Background(), "sess-1", "never finishes")
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	if !svc.Cancel(runID) {
		t.Fatal("cancel of an active run must report true")
	}
	if svc.Cancel("run-unknown") {
		t.Fatal("cancel of an unknown run must report false")
	}
	waitForRunStatus(t, backend, runID, domain.RunCancelled)

	events := replayAll(t, backend, runID)
	last := events[len(events)-1]
	if last.Type != domain.EventRunCancelled {
		t.Fatalf("last event = %s, want run.cancelled", last.Type)
	}
	if reason := payloadReasonOf(t, last.Payload); reason != reasonUserRequested {
		t.Fatalf("cancel reason = %q, want %q", reason, reasonUserRequested)
	}
	if n := countTerminal(events); n != 1 {
		t.Fatalf("terminal events = %d, want exactly 1", n)
	}

	// The journal freezes at the close: no event may be appended after the
	// terminal lands (AS-5).
	time.Sleep(100 * time.Millisecond)
	if again := replayAll(t, backend, runID); len(again) != len(events) {
		t.Fatalf("journal grew from %d to %d events after the terminal", len(events), len(again))
	}
}

func TestDeleteSessionSealsActiveRunAgainstResurrection(t *testing.T) {
	svc, backend, _ := newTestService(t, blockingModel{})
	ctx := context.Background()
	sessionID := domain.SessionID("sess-delete-active")
	if err := backend.CreateSession(ctx, domain.Session{ID: sessionID, Title: "delete me", CreatedAt: time.Now().UnixMilli()}); err != nil {
		t.Fatal(err)
	}
	runID, err := svc.Run(ctx, sessionID, "never finishes")
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if err := svc.DeleteSession(ctx, sessionID); err != nil {
		t.Fatalf("delete session: %v", err)
	}
	drainCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if !svc.WaitIdle(drainCtx) {
		t.Fatal("deleted session run did not drain")
	}
	if _, err := backend.GetRun(ctx, runID); !errors.Is(err, storage.ErrNotFound) {
		t.Fatalf("deleted run lookup error = %v, want not found", err)
	}
	msgs, err := backend.ListMessages(ctx, sessionID)
	if err != nil {
		t.Fatal(err)
	}
	if len(msgs) != 0 {
		t.Fatalf("deleted session was resurrected with messages: %+v", msgs)
	}
	if _, err := svc.Run(ctx, sessionID, "must stay deleted"); !errors.Is(err, storage.ErrNotFound) {
		t.Fatalf("run after deletion error = %v, want not found", err)
	}
}

func TestDeleteSessionRejectsLateExternalWorkerEvent(t *testing.T) {
	svc, backend, _ := newTestService(t, testsupport.NewEchoModel())
	ctx := context.Background()
	sessionID := domain.SessionID("sess-delete-worker")
	if err := backend.CreateSession(ctx, domain.Session{ID: sessionID, Title: "worker", CreatedAt: time.Now().UnixMilli()}); err != nil {
		t.Fatal(err)
	}
	child := domain.Run{
		ID: "child-delete-late", SessionID: sessionID, Status: domain.RunActive,
		CreatedAt: time.Now().UnixMilli(), Kind: domain.RunKindChild,
	}
	if err := svc.CreateWorkerRun(ctx, child); err != nil {
		t.Fatal(err)
	}
	if err := svc.DeleteSession(ctx, sessionID); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.RecordExternalRunEvent(ctx, child.ID, domain.EventChildCompleted, map[string]any{"status": "late"}); !errors.Is(err, storage.ErrNotFound) {
		t.Fatalf("late child event error = %v, want not found", err)
	}
	if err := svc.CreateWorkerRun(ctx, domain.Run{ID: "child-after-delete", SessionID: sessionID, Status: domain.RunAccepted, CreatedAt: time.Now().UnixMilli(), Kind: domain.RunKindChild}); !errors.Is(err, storage.ErrNotFound) {
		t.Fatalf("child creation after delete error = %v, want not found", err)
	}
}

func TestDeleteSessionFencesContinuableChildTree(t *testing.T) {
	svc, backend, _ := newTestService(t, testsupport.NewEchoModel())
	ctx := context.Background()
	parentSessionID := domain.SessionID("sess-delete-continuable-parent")
	parentRunID := domain.RunID("run-delete-continuable-parent")
	childSessionID := domain.SessionID("sess-delete-continuable-child")
	childRunID := domain.RunID("run-delete-continuable-child")
	authority := domain.ChildAuthorityCeiling{
		PolicyProfile: domain.PolicyProfileDefault, PolicyHash: "policy-hash",
		SandboxMode: domain.SandboxModeWorkspaceWrite, ApprovalPolicy: domain.ApprovalPolicyAsk,
		ToolNames: []string{},
	}
	authorityDigest, err := authority.Digest()
	if err != nil {
		t.Fatal(err)
	}
	requestDigest, err := domain.ChildRequestDigest("child-create", "task", []string{})
	if err != nil {
		t.Fatal(err)
	}
	if err := backend.CreateSession(ctx, domain.Session{ID: parentSessionID, Title: "parent", CreatedAt: 1}); err != nil {
		t.Fatal(err)
	}
	if err := backend.CreateRun(ctx, domain.Run{ID: parentRunID, SessionID: parentSessionID, Status: domain.RunActive, CreatedAt: 2, RootID: parentRunID}); err != nil {
		t.Fatal(err)
	}
	input := storage.ChildSessionAdmission{
		Session: domain.Session{ID: childSessionID, Title: "child", CreatedAt: 3, UpdatedAt: 3},
		Binding: domain.ChildSessionBinding{
			ChildSessionID: childSessionID, OriginParentSessionID: parentSessionID, OriginParentRunID: parentRunID,
			AuthorizerRunID: parentRunID, InitialActivationRunID: childRunID, ActivationRunID: childRunID,
			OperationKey: "child-create", RequestDigest: requestDigest, AuthorityCeilingDigest: authorityDigest,
			AuthorityCeiling: authority, ActivationOperationKey: "child-create", ActivationRequestDigest: requestDigest,
			ActivationToolNames: []string{},
			State:               domain.ChildSessionOpen, CreatedAt: 3, UpdatedAt: 3,
		},
		Admission: storage.RunAdmission{
			Message: domain.Message{ID: "child-task-message", SessionID: childSessionID, RunID: childRunID, Role: domain.RoleUser, CreatedAt: 4, Content: "task"},
			Run:     domain.Run{ID: childRunID, SessionID: childSessionID, Status: domain.RunAccepted, CreatedAt: 4, Kind: domain.RunKindChild, ChildMode: domain.ChildModeContinuable, ParentID: parentRunID, RootID: parentRunID, Depth: 1},
			Started: domain.RunEvent{RunID: childRunID, Type: domain.EventRunStarted, CreatedAt: 4, PayloadVersion: 1, Payload: []byte(`{"provider":"fixture"}`)},
		},
	}
	if result, err := backend.CommitChildSessionAdmission(ctx, input); err != nil || !result.Created {
		t.Fatalf("child admission=%+v err=%v", result, err)
	}
	if err := svc.DeleteSession(ctx, parentSessionID); err != nil {
		t.Fatalf("delete parent session: %v", err)
	}
	if err := svc.CreateWorkerRun(ctx, domain.Run{ID: "child-after-tree-delete", SessionID: childSessionID, Status: domain.RunAccepted, CreatedAt: 5, Kind: domain.RunKindChild}); !errors.Is(err, storage.ErrNotFound) {
		t.Fatalf("child admission after parent delete = %v, want not found", err)
	}
	if _, err := svc.RecordExternalRunEvent(ctx, childRunID, domain.EventChildCompleted, map[string]any{"status": "late"}); !errors.Is(err, storage.ErrNotFound) {
		t.Fatalf("late child event after parent delete = %v, want not found", err)
	}
	if _, err := backend.GetSession(ctx, childSessionID); !errors.Is(err, storage.ErrNotFound) {
		t.Fatalf("child session after parent delete = %v, want not found", err)
	}
}

func TestServiceAdmitsAndReauthorizesChildSessionWithinStoredCeiling(t *testing.T) {
	svc, backend, sink := newTestService(t, testsupport.NewEchoModel())
	ctx := context.Background()
	workspaces, err := NewSessionWorkspaceManager(t.TempDir(), backend, backend)
	if err != nil {
		t.Fatal(err)
	}
	svc.deps.Workspaces = workspaces
	parentSessionID := domain.SessionID("sess-child-service-parent")
	firstParentRunID := domain.RunID("run-child-service-parent-first")
	prepareChildSessionAuthorizer(t, svc, backend, parentSessionID, firstParentRunID, []string{tools.EchoInfoName})
	svc.mu.Lock()
	svc.runTools[firstParentRunID][tools.WriteFileName] = struct{}{}
	svc.mu.Unlock()

	request := ChildSessionRequest{
		AuthorizerRunID: firstParentRunID, OperationKey: "create-child-op", Task: "summarize this task",
		ToolNames: []string{tools.EchoInfoName},
	}
	writeRequest := request
	writeRequest.OperationKey = "create-write-child-op"
	writeRequest.ToolNames = []string{tools.WriteFileName}
	if _, err := svc.AdmitChildSession(ctx, writeRequest); !errors.Is(err, storage.ErrChildAdmissionConflict) {
		t.Fatalf("write-capable child tools = %v, want read-only ceiling conflict", err)
	}
	created, err := svc.AdmitChildSession(ctx, request)
	if err != nil || !created.Created {
		t.Fatalf("AdmitChildSession() = %+v, %v; want created", created, err)
	}
	if created.Binding.ChildSessionID == "" || created.Binding.ChildSessionID == parentSessionID ||
		created.Run.SessionID != created.Binding.ChildSessionID || created.Run.ParentID != firstParentRunID ||
		created.Run.EffectiveChildMode() != domain.ChildModeContinuable || created.Binding.AuthorityCeiling.ToolNames[0] != tools.EchoInfoName {
		t.Fatalf("child session identity or authority = %+v", created)
	}
	messages, err := backend.ListMessages(ctx, created.Binding.ChildSessionID)
	if err != nil || len(messages) != 1 || messages[0].Content != request.Task {
		t.Fatalf("child task messages = %+v, err=%v", messages, err)
	}
	published := sink.snapshot()
	if len(published) != 2 || published[0].Type != domain.EventRunStarted || published[0].RunID != created.Run.ID ||
		published[1].Type != domain.EventChildRequested || published[1].RunID != created.Run.ID {
		t.Fatalf("published child lifecycle events = %+v", published)
	}
	retry, err := svc.AdmitChildSession(ctx, request)
	if err != nil || retry.Created || retry.Binding.ChildSessionID != created.Binding.ChildSessionID || retry.Run.ID != created.Run.ID {
		t.Fatalf("idempotent child admission = %+v, %v", retry, err)
	}
	changed := request
	changed.Task = "a different task"
	if _, err := svc.AdmitChildSession(ctx, changed); !errors.Is(err, storage.ErrChildAdmissionConflict) {
		t.Fatalf("changed child payload under same operation key = %v, want conflict", err)
	}

	if err := backend.SetRunStatus(ctx, firstParentRunID, domain.RunCompleted); err != nil {
		t.Fatal(err)
	}
	secondParentRunID := domain.RunID("run-child-service-parent-second")
	prepareChildSessionAuthorizer(t, svc, backend, parentSessionID, secondParentRunID, []string{tools.EchoInfoName})
	continuation := ChildSessionContinuationRequest{
		ChildSessionID: created.Binding.ChildSessionID, AuthorizerRunID: secondParentRunID,
		OperationKey: "continue-child-op", Task: "continue with this task", ToolNames: []string{tools.EchoInfoName},
	}
	if _, err := svc.AdmitChildSessionActivation(ctx, continuation); !errors.Is(err, storage.ErrChildAdmissionConflict) {
		t.Fatalf("activation while previous child run is active = %v, want conflict", err)
	}
	if err := backend.SetRunStatus(ctx, created.Run.ID, domain.RunCompleted); err != nil {
		t.Fatal(err)
	}
	activated, err := svc.AdmitChildSessionActivation(ctx, continuation)
	if err != nil || !activated.Created {
		t.Fatalf("AdmitChildSessionActivation() = %+v, %v; want created", activated, err)
	}
	if activated.Binding.ChildSessionID != created.Binding.ChildSessionID || activated.Binding.OriginParentRunID != firstParentRunID ||
		activated.Binding.AuthorizerRunID != secondParentRunID || activated.Binding.ActivationRunID != activated.Run.ID ||
		activated.Run.ParentID != secondParentRunID {
		t.Fatalf("continuation lineage = %+v", activated)
	}
	continuedRetry, err := svc.AdmitChildSessionActivation(ctx, continuation)
	if err != nil || continuedRetry.Created || continuedRetry.Run.ID != activated.Run.ID {
		t.Fatalf("idempotent child activation = %+v, %v", continuedRetry, err)
	}
	widened := continuation
	widened.OperationKey = "widen-child-op"
	widened.ToolNames = []string{"shell"}
	if _, err := svc.AdmitChildSessionActivation(ctx, widened); !errors.Is(err, storage.ErrChildAdmissionConflict) {
		t.Fatalf("wider activation tools = %v, want conflict", err)
	}
	if err := svc.DeleteSession(ctx, parentSessionID); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.AdmitChildSessionActivation(ctx, continuation); !errors.Is(err, storage.ErrNotFound) {
		t.Fatalf("continuation after parent deletion = %v, want not found", err)
	}
}

func TestChildActivationUsesNativeRunnerWithCleanContextAndMailboxSafePoint(t *testing.T) {
	model := &captureDomainModel{inner: testsupport.NewEchoModel()}
	svc, backend, _ := newTestService(t, model)
	ctx := context.Background()
	workspaces, err := NewSessionWorkspaceManager(t.TempDir(), backend, backend)
	if err != nil {
		t.Fatal(err)
	}
	svc.deps.Workspaces = workspaces
	parentSessionID := domain.SessionID("sess-native-child-parent")
	parentRunID := domain.RunID("run-native-child-parent")
	prepareChildSessionAuthorizer(t, svc, backend, parentSessionID, parentRunID, []string{tools.EchoInfoName})
	if err := backend.AppendMessage(ctx, domain.Message{
		ID: "native-child-parent-secret", SessionID: parentSessionID, RunID: parentRunID,
		Role: domain.RoleUser, CreatedAt: time.Now().UnixMilli(), Content: "parent-only secret context",
	}); err != nil {
		t.Fatal(err)
	}
	created, err := svc.AdmitChildSession(ctx, ChildSessionRequest{
		AuthorizerRunID: parentRunID, OperationKey: "native-child-task", Task: "inspect the task input",
		ToolNames: []string{tools.EchoInfoName},
	})
	if err != nil {
		t.Fatalf("admit child: %v", err)
	}
	mail, inserted, err := svc.SendChildMessage(ctx, ChildMessageSendRequest{
		ChildSessionID: created.Binding.ChildSessionID, AuthorizerRunID: parentRunID,
		IdempotencyKey: "native-child-mail", Body: []byte("admitted parent note"),
	})
	if err != nil || !inserted {
		t.Fatalf("admit child mail = %+v inserted=%v err=%v", mail, inserted, err)
	}

	if err := svc.StartChildActivation(ctx, created.Binding.ChildSessionID, created.Run.ID); err != nil {
		t.Fatalf("start native child activation: %v", err)
	}
	t.Cleanup(func() {
		svc.CancelAll()
		svc.WaitIdle(context.Background())
	})
	waitForRunStatus(t, backend, created.Run.ID, domain.RunCompleted)

	inputs := model.snapshot()
	if len(inputs) != 1 {
		t.Fatalf("native model calls = %d, want one", len(inputs))
	}
	var userMessages []string
	for _, message := range inputs[0] {
		if message.Role == domain.RoleUser {
			if strings.HasPrefix(message.Content, "<available-deferred-tools>") {
				continue
			}
			userMessages = append(userMessages, message.Content)
		}
		if strings.Contains(message.Content, "parent-only secret context") {
			t.Fatalf("child inherited parent transcript in model input: %+v", inputs[0])
		}
		if strings.Contains(message.Content, "main agent") || strings.Contains(message.Content, "persona") {
			t.Fatalf("child inherited agent personality in model input: %+v", inputs[0])
		}
	}
	if len(userMessages) != 2 || userMessages[0] != "inspect the task input" ||
		!strings.Contains(userMessages[1], string(mail.ID)) || !strings.HasSuffix(userMessages[1], "admitted parent note") {
		t.Fatalf("child user context = %q, want task then admitted mailbox message", userMessages)
	}
	childMessages, err := backend.ListMessages(ctx, created.Binding.ChildSessionID)
	if err != nil {
		t.Fatal(err)
	}
	for _, message := range childMessages {
		if message.SessionID != created.Binding.ChildSessionID || strings.Contains(message.Content, "parent-only secret context") {
			t.Fatalf("child message projection escaped clean session scope: %+v", childMessages)
		}
	}
	if len(childMessages) < 2 || childMessages[0].Content != "inspect the task input" || childMessages[len(childMessages)-1].Role != domain.RoleAssistant {
		t.Fatalf("child session transcript = %+v, want task and native runner response", childMessages)
	}
	if got, err := backend.GetChildMessageReceipt(ctx, created.Binding.ChildSessionID, string(mail.ID), created.Run.ID); err != nil || got.State != domain.ChildMessageReceiptConsumed {
		t.Fatalf("mailbox safe-point receipt = %+v err=%v, want consumed", got, err)
	}
	updated, err := backend.GetChildSessionBinding(ctx, created.Binding.ChildSessionID)
	if err != nil || updated.ConsumedMessageSequence != mail.Sequence {
		t.Fatalf("consumed child inbox cursor = %+v err=%v, want %d", updated, err, mail.Sequence)
	}
	var sawNativeLifecycle bool
	for _, event := range replayAll(t, backend, created.Run.ID) {
		if event.Type == domain.EventModelRequest {
			sawNativeLifecycle = true
		}
	}
	if !sawNativeLifecycle {
		t.Fatal("child activation did not run through Service model request journaling")
	}
	childEvents := replayAll(t, backend, created.Run.ID)
	if countTerminal(childEvents) != 1 || indexOfType(childEvents, domain.EventChildRequested) < 0 ||
		indexOfType(childEvents, domain.EventChildStarted) < 0 || indexOfType(childEvents, domain.EventChildCompleted) < 0 {
		t.Fatalf("continuable child did not persist exactly one child terminal: %+v", childEvents)
	}
}

func TestOneShotChildCannotSelectEffectfulParentTool(t *testing.T) {
	svc, backend, _ := newTestService(t, testsupport.NewEchoModel())
	ctx := context.Background()
	registered, err := tools.Builtin(backend).Resolve([]string{tools.EchoInfoName, tools.WriteNoteName})
	if err != nil {
		t.Fatal(err)
	}
	engine, err := NewEngine(ctx, WrapModel(testsupport.NewEchoModel()), registered, EngineConfig{StreamBuffer: 8, MaxEventPayloadBytes: 64 << 10})
	if err != nil {
		t.Fatal(err)
	}
	svc.engine = engine
	workspaces, err := NewSessionWorkspaceManager(t.TempDir(), backend, backend)
	if err != nil {
		t.Fatal(err)
	}
	svc.deps.Workspaces = workspaces
	parentSessionID := domain.SessionID("sess-readonly-child-parent")
	parentRunID := domain.RunID("run-readonly-child-parent")
	prepareChildSessionAuthorizer(t, svc, backend, parentSessionID, parentRunID, []string{tools.EchoInfoName})
	if _, err := svc.StartOneShotChild(ctx, OneShotChildRequest{
		ParentRunID: parentRunID, Task: "try to write", ToolNames: []string{tools.WriteNoteName},
	}); !errors.Is(err, storage.ErrChildAdmissionConflict) {
		t.Fatalf("one-shot child selected an effectful tool: %v, want authority conflict", err)
	}
	children, err := backend.ListChildRuns(ctx, parentRunID)
	if err != nil || len(children) != 0 {
		t.Fatalf("rejected child left persisted Runs: %+v err=%v", children, err)
	}
}

func TestChildrenCannotSelectHumanInteractionTool(t *testing.T) {
	svc, backend, _ := newTestService(t, testsupport.NewEchoModel())
	ctx := context.Background()
	registered, err := tools.Builtin(backend).Resolve([]string{tools.EchoInfoName, tools.AskUserName})
	if err != nil {
		t.Fatal(err)
	}
	engine, err := NewEngine(ctx, WrapModel(testsupport.NewEchoModel()), registered, EngineConfig{StreamBuffer: 8, MaxEventPayloadBytes: 64 << 10})
	if err != nil {
		t.Fatal(err)
	}
	svc.engine = engine
	workspaces, err := NewSessionWorkspaceManager(t.TempDir(), backend, backend)
	if err != nil {
		t.Fatal(err)
	}
	svc.deps.Workspaces = workspaces
	parentSessionID := domain.SessionID("sess-headless-child-parent")
	parentRunID := domain.RunID("run-headless-child-parent")
	prepareChildSessionAuthorizer(t, svc, backend, parentSessionID, parentRunID, []string{tools.EchoInfoName, tools.AskUserName})

	// A continuable child inherits the parent ceiling minus headless-unsafe
	// tools: ask_user must be dropped even when the parent holds it.
	created, err := svc.AdmitChildSession(ctx, ChildSessionRequest{
		AuthorizerRunID: parentRunID, OperationKey: "headless-child-op", Task: "summarize this task",
	})
	if err != nil || !created.Created {
		t.Fatalf("AdmitChildSession() = %+v, %v; want created", created, err)
	}
	for _, name := range created.Binding.AuthorityCeiling.ToolNames {
		if name == tools.AskUserName {
			t.Fatalf("child ceiling holds human-interaction tool: %+v", created.Binding.AuthorityCeiling.ToolNames)
		}
	}
	if len(created.Binding.AuthorityCeiling.ToolNames) != 1 || created.Binding.AuthorityCeiling.ToolNames[0] != tools.EchoInfoName {
		t.Fatalf("child ceiling = %+v, want echo_info only", created.Binding.AuthorityCeiling.ToolNames)
	}
	if _, err := svc.AdmitChildSession(ctx, ChildSessionRequest{
		AuthorizerRunID: parentRunID, OperationKey: "ask-user-child-op", Task: "ask the user",
		ToolNames: []string{tools.AskUserName},
	}); !errors.Is(err, storage.ErrChildAdmissionConflict) {
		t.Fatalf("continuable child selected ask_user: %v, want authority conflict", err)
	}
	if _, err := svc.StartOneShotChild(ctx, OneShotChildRequest{
		ParentRunID: parentRunID, Task: "ask the user", ToolNames: []string{tools.AskUserName},
	}); err == nil {
		t.Fatal("one-shot child selected ask_user; want rejection")
	}
}

func TestOneShotChildUsesNativeRunnerWithoutCreatingAddressableSessionOrParentMessages(t *testing.T) {
	model := &captureDomainModel{inner: testsupport.NewEchoModel()}
	svc, backend, _ := newTestService(t, model)
	ctx := context.Background()
	workspaces, err := NewSessionWorkspaceManager(t.TempDir(), backend, backend)
	if err != nil {
		t.Fatal(err)
	}
	svc.deps.Workspaces = workspaces
	svc.deps.Admission = backend
	svc.deps.GenerationID = "native-one-shot-generation"
	parentSessionID := domain.SessionID("sess-native-one-shot-parent")
	parentRunID := domain.RunID("run-native-one-shot-parent")
	prepareChildSessionAuthorizer(t, svc, backend, parentSessionID, parentRunID, []string{tools.EchoInfoName})
	if err := backend.AppendMessage(ctx, domain.Message{
		ID: "native-one-shot-parent-secret", SessionID: parentSessionID, RunID: parentRunID,
		Role: domain.RoleUser, CreatedAt: time.Now().UnixMilli(), Content: "parent-only secret context",
	}); err != nil {
		t.Fatal(err)
	}

	started, err := svc.StartOneShotChild(ctx, OneShotChildRequest{ParentRunID: parentRunID, Task: "one shot clean task"})
	if err != nil {
		t.Fatalf("start native one-shot child: %v", err)
	}
	if started.Run.EffectiveChildMode() != domain.ChildModeOneShot || started.Run.SessionID != parentSessionID || started.Run.ParentID != parentRunID {
		t.Fatalf("one-shot lineage = %+v", started.Run)
	}
	svc.WaitIdle(ctx)
	waitForRunStatus(t, backend, started.Run.ID, domain.RunCompleted)
	if _, err := backend.LoadRunPrompt(ctx, started.Run.ID); err != nil {
		t.Fatalf("load durable one-shot prompt snapshot: %v", err)
	}
	summary, failure, err := svc.OneShotChildOutcome(ctx, started.Run.ID)
	if err != nil || failure != "" || summary != "test response to: one shot clean task" {
		t.Fatalf("one-shot outcome = %q failure=%q err=%v", summary, failure, err)
	}
	children, err := backend.ListChildSessions(ctx, parentSessionID)
	if err != nil || len(children) != 0 {
		t.Fatalf("one-shot task created addressable ChildSessions = %+v err=%v", children, err)
	}
	parentMessages, err := backend.ListMessages(ctx, parentSessionID)
	if err != nil {
		t.Fatal(err)
	}
	if len(parentMessages) != 1 || parentMessages[0].Content != "parent-only secret context" {
		t.Fatalf("one-shot messages leaked into parent transcript: %+v", parentMessages)
	}
	inputs := model.snapshot()
	if len(inputs) != 1 {
		t.Fatalf("one-shot model calls = %d, want one", len(inputs))
	}
	var userMessages []string
	for _, message := range inputs[0] {
		if strings.Contains(message.Content, "parent-only secret context") {
			t.Fatalf("one-shot child inherited parent context: %+v", inputs[0])
		}
		if message.Role == domain.RoleUser && !strings.HasPrefix(message.Content, "<available-deferred-tools>") {
			userMessages = append(userMessages, message.Content)
		}
	}
	if !reflect.DeepEqual(userMessages, []string{"one shot clean task"}) {
		t.Fatalf("one-shot model task input = %q", userMessages)
	}
}

type captureDomainModel struct {
	inner  domain.ChatModel
	mu     sync.Mutex
	inputs [][]*domain.Message
}

func (m *captureDomainModel) Stream(ctx context.Context, input []*domain.Message) (domain.Stream[*domain.Message], error) {
	copyInput := make([]*domain.Message, 0, len(input))
	for _, message := range input {
		if message == nil {
			continue
		}
		clone := *message
		copyInput = append(copyInput, &clone)
	}
	m.mu.Lock()
	m.inputs = append(m.inputs, copyInput)
	m.mu.Unlock()
	return m.inner.Stream(ctx, input)
}

func (m *captureDomainModel) snapshot() [][]*domain.Message {
	m.mu.Lock()
	defer m.mu.Unlock()
	inputs := make([][]*domain.Message, len(m.inputs))
	for i, input := range m.inputs {
		inputs[i] = append([]*domain.Message(nil), input...)
	}
	return inputs
}

func TestServiceChildMailboxDerivesDirectParticipantAndPersistsInboxCursors(t *testing.T) {
	svc, backend, _ := newTestService(t, testsupport.NewEchoModel())
	ctx := context.Background()
	workspaces, err := NewSessionWorkspaceManager(t.TempDir(), backend, backend)
	if err != nil {
		t.Fatal(err)
	}
	svc.deps.Workspaces = workspaces
	parentSessionID := domain.SessionID("sess-child-mail-service-parent")
	parentRunID := domain.RunID("run-child-mail-service-parent")
	prepareChildSessionAuthorizer(t, svc, backend, parentSessionID, parentRunID, []string{tools.EchoInfoName})
	created, err := svc.AdmitChildSession(ctx, ChildSessionRequest{
		AuthorizerRunID: parentRunID, OperationKey: "mail-child-admission", Task: "task",
	})
	if err != nil {
		t.Fatalf("admit child: %v", err)
	}
	childID := created.Binding.ChildSessionID

	request := ChildMessageSendRequest{
		ChildSessionID: childID, AuthorizerRunID: parentRunID, IdempotencyKey: "parent-mail-1", Body: []byte("parent to child"),
	}
	first, inserted, err := svc.SendChildMessage(ctx, request)
	if err != nil || !inserted || first.SenderSessionID != parentSessionID || first.RecipientSessionID != childID || first.Sequence != 1 {
		t.Fatalf("parent send = %+v inserted=%v err=%v", first, inserted, err)
	}
	retry, inserted, err := svc.SendChildMessage(ctx, request)
	if err != nil || inserted || retry.ID != first.ID {
		t.Fatalf("idempotent parent send = %+v inserted=%v err=%v", retry, inserted, err)
	}
	changed := request
	changed.Body = []byte("changed body")
	if _, _, err := svc.SendChildMessage(ctx, changed); !errors.Is(err, storage.ErrChildMessageConflict) {
		t.Fatalf("changed body under same idempotency key = %v, want conflict", err)
	}

	childInbox, err := svc.ListChildMessages(ctx, childID, created.Run.ID, 10)
	if err != nil || len(childInbox) != 1 || childInbox[0].ID != first.ID {
		t.Fatalf("child inbox = %+v err=%v", childInbox, err)
	}
	if _, _, err := svc.RecordChildMessageReceipt(ctx, childID, parentRunID, first.ID, domain.ChildMessageReceiptConsumed); !errors.Is(err, storage.ErrChildAdmissionConflict) {
		t.Fatalf("parent run consumed child inbox item = %v, want conflict", err)
	}
	progress, inserted, err := svc.RecordChildMessageReceipt(ctx, childID, created.Run.ID, first.ID, domain.ChildMessageReceiptInProgress)
	if err != nil || !inserted || progress.ConsumerRunID != created.Run.ID {
		t.Fatalf("child in-progress receipt = %+v inserted=%v err=%v", progress, inserted, err)
	}
	consumed, inserted, err := svc.RecordChildMessageReceipt(ctx, childID, created.Run.ID, first.ID, domain.ChildMessageReceiptConsumed)
	if err != nil || !inserted || consumed.State != domain.ChildMessageReceiptConsumed {
		t.Fatalf("child consumed receipt = %+v inserted=%v err=%v", consumed, inserted, err)
	}
	childInbox, err = svc.ListChildMessages(ctx, childID, created.Run.ID, 10)
	if err != nil || len(childInbox) != 0 {
		t.Fatalf("child inbox after receipt = %+v err=%v", childInbox, err)
	}

	reply, inserted, err := svc.SendChildMessage(ctx, ChildMessageSendRequest{
		ChildSessionID: childID, AuthorizerRunID: created.Run.ID, IdempotencyKey: "child-mail-1", Body: []byte("child to parent"),
	})
	if err != nil || !inserted || reply.SenderSessionID != childID || reply.RecipientSessionID != parentSessionID || reply.Sequence != 1 {
		t.Fatalf("child reply = %+v inserted=%v err=%v", reply, inserted, err)
	}
	parentInbox, err := svc.ListChildMessages(ctx, childID, parentRunID, 10)
	if err != nil || len(parentInbox) != 1 || parentInbox[0].ID != reply.ID {
		t.Fatalf("parent inbox = %+v err=%v", parentInbox, err)
	}
	if _, _, err := svc.RecordChildMessageReceipt(ctx, childID, created.Run.ID, reply.ID, domain.ChildMessageReceiptConsumed); !errors.Is(err, storage.ErrChildAdmissionConflict) {
		t.Fatalf("child run consumed parent inbox item = %v, want conflict", err)
	}
	if _, inserted, err := svc.RecordChildMessageReceipt(ctx, childID, parentRunID, reply.ID, domain.ChildMessageReceiptConsumed); err != nil || !inserted {
		t.Fatalf("parent consumed receipt inserted=%v err=%v", inserted, err)
	}

	// A later Run in the same parent Session is a fresh authorization boundary.
	// It can inspect and consume the durable child reply without first creating
	// an unrelated child activation that rotates the binding's authorizer.
	// The authorizer run is terminal: one session admits one active primary.
	if err := backend.SetRunStatus(ctx, parentRunID, domain.RunCompleted); err != nil {
		t.Fatal(err)
	}
	reauthorizedRunID := domain.RunID("run-child-mail-service-reauthorized-parent")
	prepareChildSessionAuthorizer(t, svc, backend, parentSessionID, reauthorizedRunID, []string{tools.EchoInfoName})
	reply, inserted, err = svc.SendChildMessage(ctx, ChildMessageSendRequest{
		ChildSessionID: childID, AuthorizerRunID: created.Run.ID, IdempotencyKey: "child-mail-2", Body: []byte("second reply"),
	})
	if err != nil || !inserted {
		t.Fatalf("second child reply = %+v inserted=%v err=%v", reply, inserted, err)
	}
	parentInbox, err = svc.ListChildMessages(ctx, childID, reauthorizedRunID, 10)
	if err != nil || len(parentInbox) != 1 || parentInbox[0].ID != reply.ID {
		t.Fatalf("reauthorized parent inbox=%+v err=%v", parentInbox, err)
	}
	if _, inserted, err := svc.RecordChildMessageReceipt(ctx, childID, reauthorizedRunID, reply.ID, domain.ChildMessageReceiptConsumed); err != nil || !inserted {
		t.Fatalf("reauthorized parent consumed receipt inserted=%v err=%v", inserted, err)
	}

	thirdReply, inserted, err := svc.SendChildMessage(ctx, ChildMessageSendRequest{
		ChildSessionID: childID, AuthorizerRunID: created.Run.ID, IdempotencyKey: "child-mail-3", Body: []byte("ready for parent")})
	if err != nil || !inserted {
		t.Fatalf("third child reply = %+v inserted=%v err=%v", thirdReply, inserted, err)
	}
	deliveries, err := svc.pendingParentReplies(ctx, parentSessionID, reauthorizedRunID)
	if err != nil || len(deliveries) != 1 || deliveries[0].message.ID != thirdReply.ID {
		t.Fatalf("pending parent replies=%+v err=%v", deliveries, err)
	}
	formatted := formatParentReplies(deliveries, "what happened?")
	if !strings.Contains(formatted, "ready for parent") || !strings.Contains(formatted, "what happened?") {
		t.Fatalf("formatted parent input omitted reply or request: %q", formatted)
	}
	toolInbox, err := svc.ReadParentInbox(ctx, reauthorizedRunID, parentSessionID)
	if err != nil || len(toolInbox) != 1 || toolInbox[0].Text != "ready for parent" {
		t.Fatalf("child_inbox tool read=%+v err=%v", toolInbox, err)
	}
	if err := svc.consumeParentReplies(ctx, deliveries, reauthorizedRunID); err != nil {
		t.Fatalf("consume parent replies at completion: %v", err)
	}
	parentInbox, err = svc.ListChildMessages(ctx, childID, reauthorizedRunID, 10)
	if err != nil || len(parentInbox) != 0 {
		t.Fatalf("parent inbox after completion safe point=%+v err=%v", parentInbox, err)
	}
	otherSessionID := domain.SessionID("sess-child-mail-service-unrelated")
	otherRunID := domain.RunID("run-child-mail-service-unrelated")
	if err := backend.CreateSession(ctx, domain.Session{ID: otherSessionID, Title: "unrelated", CreatedAt: 20}); err != nil {
		t.Fatal(err)
	}
	if err := backend.CreateRun(ctx, domain.Run{ID: otherRunID, SessionID: otherSessionID, Status: domain.RunActive, CreatedAt: 21}); err != nil {
		t.Fatal(err)
	}
	if _, _, err := svc.SendChildMessage(ctx, ChildMessageSendRequest{ChildSessionID: childID, AuthorizerRunID: otherRunID, IdempotencyKey: "forged", Body: []byte("forged")}); !errors.Is(err, storage.ErrChildAdmissionConflict) {
		t.Fatalf("unrelated run sent child mail = %v, want conflict", err)
	}
	if err := backend.CloseChildSession(ctx, childID, time.Now().UnixMilli()); err != nil {
		t.Fatalf("close child for historical view: %v", err)
	}
	if err := backend.SetRunStatus(ctx, parentRunID, domain.RunCompleted); err != nil {
		t.Fatalf("finish historical authorizer: %v", err)
	}
	if _, err := svc.ChildSessionHistory(ctx, childID, parentRunID); err != nil {
		t.Fatalf("historical child view after close/parent completion: %v", err)
	}
}

func TestParentRunReceivesChildReplyInModelInputAndConsumesAtCompletion(t *testing.T) {
	ctx := context.Background()
	backend, err := sqlite.Open(ctx, filepath.Join(t.TempDir(), "parent-inbox-run.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = backend.Close() })
	resolved, err := tools.Builtin(backend).Resolve([]string{tools.EchoInfoName})
	if err != nil {
		t.Fatal(err)
	}
	recorder := &recordingChatModel{inner: NewScriptedModel(schema.AssistantMessage("parent done", nil))}
	engine, err := NewEngine(ctx, recorder, resolved, EngineConfig{StreamBuffer: 8, MaxEventPayloadBytes: 64 << 10})
	if err != nil {
		t.Fatal(err)
	}
	svc := NewService(engine, "test", "test-model", ServiceDeps{Journal: backend, Runs: backend, Messages: backend, Sessions: backend, Sink: newTestSink()})
	svc.deps.Workspaces = fixedWorkspaceAllocator{}
	const parentSessionID domain.SessionID = "session-parent-reply-run"
	const parentRunID domain.RunID = "run-parent-reply-authorizer"
	prepareChildSessionAuthorizer(t, svc, backend, parentSessionID, parentRunID, []string{tools.EchoInfoName})
	const childSessionID domain.SessionID = "session-parent-reply-child"
	const childRunID domain.RunID = "run-parent-reply-child"
	parentSession, err := backend.GetSession(ctx, parentSessionID)
	if err != nil {
		t.Fatal(err)
	}
	parentRunRecord, err := backend.GetRun(ctx, parentRunID)
	if err != nil {
		t.Fatal(err)
	}
	snapshot, err := svc.engine.cfg.Policy.Snapshot(domain.PolicyProfileDefault)
	if err != nil {
		t.Fatal(err)
	}
	sandbox, approval := parentSession.EffectiveSandbox()
	authority := domain.ChildAuthorityCeiling{PolicyProfile: snapshot.Profile, PolicyHash: snapshot.Hash, SandboxMode: sandbox, ApprovalPolicy: approval, ToolNames: []string{tools.EchoInfoName}}
	authorityDigest, err := authority.Digest()
	if err != nil {
		t.Fatal(err)
	}
	requestDigest, err := domain.ChildRequestDigest("reply-run-child", "compute a result", authority.ToolNames)
	if err != nil {
		t.Fatal(err)
	}
	createdAt := time.Now().UnixMilli()
	childAdmission := storage.ChildSessionAdmission{
		Session:   domain.Session{ID: childSessionID, Title: "child", CreatedAt: createdAt, UpdatedAt: createdAt, SandboxMode: string(sandbox), ApprovalPolicy: string(approval)},
		Binding:   domain.ChildSessionBinding{ChildSessionID: childSessionID, OriginParentSessionID: parentSessionID, OriginParentRunID: parentRunID, AuthorizerRunID: parentRunID, InitialActivationRunID: childRunID, ActivationRunID: childRunID, OperationKey: "reply-run-child", RequestDigest: requestDigest, AuthorityCeilingDigest: authorityDigest, AuthorityCeiling: authority, ActivationOperationKey: "reply-run-child", ActivationRequestDigest: requestDigest, ActivationToolNames: authority.ToolNames, State: domain.ChildSessionOpen, CreatedAt: createdAt, UpdatedAt: createdAt},
		Admission: storage.RunAdmission{Message: domain.Message{ID: "message-parent-reply-child", SessionID: childSessionID, RunID: childRunID, Role: domain.RoleUser, CreatedAt: createdAt, Content: "compute a result"}, Run: domain.Run{ID: childRunID, SessionID: childSessionID, Status: domain.RunAccepted, CreatedAt: createdAt, Kind: domain.RunKindChild, ChildMode: domain.ChildModeContinuable, ParentID: parentRunID, RootID: parentRunRecord.RootID, Depth: 1}, Started: domain.RunEvent{RunID: childRunID, Type: domain.EventRunStarted, CreatedAt: createdAt, PayloadVersion: 1, Payload: []byte(`{"provider":"test"}`)}},
	}
	if _, err := backend.CommitChildSessionAdmission(ctx, childAdmission); err != nil {
		t.Fatalf("persist child fixture: %v", err)
	}
	if err := backend.SetRunStatus(ctx, childRunID, domain.RunActive); err != nil {
		t.Fatal(err)
	}
	message, inserted, err := backend.EnqueueChildMessage(ctx, domain.ChildMailboxMessage{
		ID: "mail-parent-reply-run", ChildSessionID: childSessionID,
		SenderSessionID: childSessionID, RecipientSessionID: parentSessionID,
		IdempotencyKey: "reply-run-mail", Body: []byte("the result is 42"), CreatedAt: time.Now().UnixMilli(),
	})
	if err != nil || !inserted || message.ID == "" {
		t.Fatalf("enqueue child reply=%+v inserted=%v err=%v", message, inserted, err)
	}
	// The authorizer turn has ended; the new turn is the session's active
	// primary run.
	if err := backend.SetRunStatus(ctx, parentRunID, domain.RunCompleted); err != nil {
		t.Fatal(err)
	}
	parentRun, err := svc.Run(ctx, parentSessionID, "tell me the child result")
	if err != nil {
		t.Fatalf("start parent run: %v", err)
	}
	waitForRunStatus(t, backend, parentRun, domain.RunCompleted)
	feed := recorder.lastInput()
	var input strings.Builder
	for _, msg := range feed {
		if msg != nil {
			input.WriteString(msg.Content)
			input.WriteByte('\n')
		}
	}
	if !strings.Contains(input.String(), "the result is 42") || !strings.Contains(input.String(), "tell me the child result") {
		t.Fatalf("parent model input did not include the child reply and current request: %q", input.String())
	}
	binding, err := backend.GetChildSessionBinding(ctx, childSessionID)
	if err != nil || binding.ConsumedParentMessageSequence != 1 {
		t.Fatalf("parent inbox cursor=%d err=%v", binding.ConsumedParentMessageSequence, err)
	}
	if binding.AuthorizerRunID != parentRunID {
		t.Fatalf("ordinary parent run unexpectedly rotated child authorizer to %q", binding.AuthorizerRunID)
	}
}

func prepareChildSessionAuthorizer(t *testing.T, svc *Service, backend *sqlite.Backend, sessionID domain.SessionID, runID domain.RunID, selected []string) {
	t.Helper()
	ctx := context.Background()
	if _, err := backend.GetSession(ctx, sessionID); errors.Is(err, storage.ErrNotFound) {
		if err := backend.CreateSession(ctx, domain.Session{ID: sessionID, Title: "parent", CreatedAt: time.Now().UnixMilli()}); err != nil {
			t.Fatal(err)
		}
	} else if err != nil {
		t.Fatal(err)
	}
	if err := backend.CreateRun(ctx, domain.Run{ID: runID, SessionID: sessionID, Status: domain.RunActive, CreatedAt: time.Now().UnixMilli(), Kind: domain.RunKindPrimary, RootID: "run-child-service-parent-first"}); err != nil {
		t.Fatal(err)
	}
	snapshot, err := svc.engine.cfg.Policy.Snapshot(domain.PolicyProfileDefault)
	if err != nil {
		t.Fatal(err)
	}
	ledger, err := NewBudgetLedger(svc.deps.Budget)
	if err != nil {
		t.Fatal(err)
	}
	toolSet := childToolSet(selected)
	svc.mu.Lock()
	svc.snapshots[runID] = snapshot
	svc.ledgers[runID] = ledger
	svc.runTools[runID] = toolSet
	svc.runSessions[runID] = sessionID
	svc.mu.Unlock()
}

func TestDeleteSessionRacesWorkerCreationWithoutOrphans(t *testing.T) {
	svc, backend, _ := newTestService(t, testsupport.NewEchoModel())
	ctx := context.Background()
	sessionID := domain.SessionID("sess-delete-worker-race")
	if err := backend.CreateSession(ctx, domain.Session{ID: sessionID, Title: "worker race", CreatedAt: time.Now().UnixMilli()}); err != nil {
		t.Fatal(err)
	}
	start := make(chan struct{})
	var wg sync.WaitGroup
	for i := 0; i < 32; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start
			_ = svc.CreateWorkerRun(ctx, domain.Run{
				ID: domain.RunID(fmt.Sprintf("child-delete-race-%02d", i)), SessionID: sessionID,
				Status: domain.RunAccepted, CreatedAt: time.Now().UnixMilli(), Kind: domain.RunKindChild,
			})
		}(i)
	}
	close(start)
	if err := svc.DeleteSession(ctx, sessionID); err != nil {
		t.Fatal(err)
	}
	wg.Wait()
	runs, err := backend.ListRunsBySession(ctx, sessionID)
	if err != nil {
		t.Fatal(err)
	}
	if len(runs) != 0 {
		t.Fatalf("worker/delete race left orphan runs: %+v", runs)
	}
}

// The request context must not own the run: cancelling it (RPC disconnect,
// page refresh) leaves the run alive until Cancel is called (AS-7).
func TestServiceRunSurvivesRequestCancellation(t *testing.T) {
	svc, backend, _ := newTestService(t, blockingModel{})
	mustCreateSession(t, backend, "sess-1")
	ctx, cancel := context.WithCancel(context.Background())
	runID, err := svc.Run(ctx, "sess-1", "keep going")
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	cancel()

	time.Sleep(50 * time.Millisecond)
	r, err := backend.GetRun(context.Background(), runID)
	if err != nil {
		t.Fatalf("get run: %v", err)
	}
	if r.Status != domain.RunActive {
		t.Fatalf("run status after request cancel = %s, want active", r.Status)
	}

	if !svc.Cancel(runID) {
		t.Fatal("cancel of an active run must report true")
	}
	waitForRunStatus(t, backend, runID, domain.RunCancelled)
}

// errorModel fails immediately, driving the run.failed path.
type errorModel struct{}

var errProviderBoom = errors.New("provider exploded")

func (errorModel) Stream(_ context.Context, _ []*domain.Message) (domain.Stream[*domain.Message], error) {
	return nil, errProviderBoom
}

func TestServiceRunFailed(t *testing.T) {
	svc, backend, _ := newTestService(t, errorModel{})
	mustCreateSession(t, backend, "sess-1")
	runID, err := svc.Run(context.Background(), "sess-1", "boom")
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	waitForRunStatus(t, backend, runID, domain.RunFailed)

	events := replayAll(t, backend, runID)
	last := events[len(events)-1]
	if last.Type != domain.EventRunFailed {
		t.Fatalf("last event = %s, want run.failed", last.Type)
	}
	cat, msg := payloadFailureOf(t, last.Payload)
	if cat != causeInternalError {
		t.Fatalf("cause category = %q, want %q", cat, causeInternalError)
	}
	if msg == "" {
		t.Fatal("run.failed must carry a user-visible message")
	}
	if strings.Contains(msg, "provider exploded") {
		t.Fatalf("failure message leaks internals: %q", msg)
	}
	if n := countTerminal(events); n != 1 {
		t.Fatalf("terminal events = %d, want exactly 1", n)
	}
}

// keyMissingModel fails with the provider's typed KeyMissingError, which the
// engine may wrap on the way out; the terminal must still classify it as a
// provider failure with an actionable message.
type keyMissingModel struct{}

func (keyMissingModel) Stream(_ context.Context, _ []*domain.Message) (domain.Stream[*domain.Message], error) {
	return nil, &provider.KeyMissingError{Provider: "deepseek"}
}

type unconfiguredModel struct{}

func (unconfiguredModel) Stream(_ context.Context, _ []*domain.Message) (domain.Stream[*domain.Message], error) {
	return nil, fmt.Errorf("resolve active model: %w", provider.ErrModelNotConfigured)
}

func TestServiceRunFailedWithoutProvider(t *testing.T) {
	svc, backend, _ := newTestService(t, unconfiguredModel{})
	mustCreateSession(t, backend, "sess-1")
	runID, err := svc.Run(context.Background(), "sess-1", "hello")
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	waitForRunStatus(t, backend, runID, domain.RunFailed)

	events := replayAll(t, backend, runID)
	last := events[len(events)-1]
	if last.Type != domain.EventRunFailed {
		t.Fatalf("last event = %s, want run.failed", last.Type)
	}
	cat, msg := payloadFailureOf(t, last.Payload)
	if cat != causeProviderError {
		t.Fatalf("cause category = %q, want %q", cat, causeProviderError)
	}
	if msg != providerUnavailableMessage {
		t.Fatalf("failure message = %q, want %q", msg, providerUnavailableMessage)
	}
}

func TestServiceRunFailedKeyMissing(t *testing.T) {
	svc, backend, _ := newTestService(t, keyMissingModel{})
	mustCreateSession(t, backend, "sess-1")
	runID, err := svc.Run(context.Background(), "sess-1", "hello")
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	waitForRunStatus(t, backend, runID, domain.RunFailed)

	events := replayAll(t, backend, runID)
	last := events[len(events)-1]
	if last.Type != domain.EventRunFailed {
		t.Fatalf("last event = %s, want run.failed", last.Type)
	}
	cat, msg := payloadFailureOf(t, last.Payload)
	if cat != causeProviderError {
		t.Fatalf("cause category = %q, want %q", cat, causeProviderError)
	}
	if msg != providerUnavailableMessage {
		t.Fatalf("failure message = %q, want %q", msg, providerUnavailableMessage)
	}
	if strings.Contains(msg, "sk-") || strings.Contains(msg, "api_key") {
		t.Fatalf("failure message leaks a key value or field: %q", msg)
	}
}

// transportErrorModel fails with a wrapped network error; the terminal must
// classify it as provider transport, not an internal mystery.
type transportErrorModel struct{}

func (transportErrorModel) Stream(_ context.Context, _ []*domain.Message) (domain.Stream[*domain.Message], error) {
	return nil, fmt.Errorf("model stream recv: %w", errors.New("dial tcp 127.0.0.1:9999: connect: connection refused"))
}

func TestServiceRunFailedProviderTransport(t *testing.T) {
	svc, backend, _ := newTestService(t, transportErrorModel{})
	mustCreateSession(t, backend, "sess-1")
	runID, err := svc.Run(context.Background(), "sess-1", "hello")
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	waitForRunStatus(t, backend, runID, domain.RunFailed)

	events := replayAll(t, backend, runID)
	last := events[len(events)-1]
	if last.Type != domain.EventRunFailed {
		t.Fatalf("last event = %s, want run.failed", last.Type)
	}
	cat, msg := payloadFailureOf(t, last.Payload)
	if cat != causeProviderError {
		t.Fatalf("cause category = %q, want %q", cat, causeProviderError)
	}
	if msg != providerUnavailableMessage {
		t.Fatalf("failure message = %q, want %q", msg, providerUnavailableMessage)
	}
	if strings.Contains(msg, "127.0.0.1") || strings.Contains(msg, "dial tcp") {
		t.Fatalf("failure message leaks transport internals: %q", msg)
	}
}

func TestMapperToolCallAndResult(t *testing.T) {
	m := newEventMapper("run-test", 0)

	callEvents, err := m.onEvent(&adk.AgentEvent{
		Output: &adk.AgentOutput{MessageOutput: &adk.TypedMessageVariant[*schema.Message]{
			Message: &schema.Message{
				Role: schema.Assistant,
				ToolCalls: []schema.ToolCall{{
					ID:       "call-1",
					Function: schema.FunctionCall{Name: "echo_info", Arguments: `{"text":"hi"}`},
				}},
			},
		}},
	})
	if err != nil {
		t.Fatalf("map tool call: %v", err)
	}
	if len(callEvents) != 1 || callEvents[0].Type != domain.EventToolRequested {
		t.Fatalf("expected one tool.requested, got %+v", callEvents)
	}
	if !strings.Contains(string(callEvents[0].Payload), `"tool_name":"echo_info"`) {
		t.Fatalf("tool.requested payload missing tool name: %s", callEvents[0].Payload)
	}
	if !strings.Contains(string(callEvents[0].Payload), `"text":"hi"`) {
		t.Fatalf("tool.requested payload missing parsed args: %s", callEvents[0].Payload)
	}

	resultEvents, err := m.onEvent(&adk.AgentEvent{
		Output: &adk.AgentOutput{MessageOutput: &adk.TypedMessageVariant[*schema.Message]{
			Message: &schema.Message{Role: schema.Tool, Content: "hi", ToolCallID: "call-1"},
		}},
	})
	if err != nil {
		t.Fatalf("map tool result: %v", err)
	}
	if len(resultEvents) != 2 {
		t.Fatalf("expected tool.started + tool.finished, got %d events", len(resultEvents))
	}
	if resultEvents[0].Type != domain.EventToolStarted || resultEvents[1].Type != domain.EventToolFinished {
		t.Fatalf("unexpected tool event order: %s, %s", resultEvents[0].Type, resultEvents[1].Type)
	}
	if !strings.Contains(string(resultEvents[1].Payload), `"result":"hi"`) {
		t.Fatalf("tool.finished payload missing result: %s", resultEvents[1].Payload)
	}
}

func TestMapperTurnEndFlushesModelCompleted(t *testing.T) {
	m := newEventMapper("run-test", 0)
	m.pendingText.WriteString("partial")
	m.hasPending = true

	events := m.onTurnEnd()
	if len(events) != 1 || events[0].Type != domain.EventModelCompleted {
		t.Fatalf("expected flushed model.completed, got %+v", events)
	}
	if events[0].PayloadVersion != 2 {
		t.Fatalf("flushed model.completed payload version = %d, want 2", events[0].PayloadVersion)
	}
	completed := payloadCompletedOf(t, events[0].Payload)
	if completed.ContentSHA256 != sha256Hex([]byte("partial")) || completed.ByteLen != len([]byte("partial")) {
		t.Fatalf("model.completed payload wrong: %+v", completed)
	}
	if again := m.onTurnEnd(); len(again) != 0 {
		t.Fatalf("second flush must be empty, got %+v", again)
	}
}

func TestMapperFlushesAssistantTextBeforeToolAndFencesNextRound(t *testing.T) {
	m := newEventMapper("run-test", 0)
	m.pendingText.WriteString("before tool")
	m.hasPending = true
	events, err := m.onMessageEvent(&adk.TypedMessageVariant[*schema.Message]{Message: &schema.Message{
		Role:      schema.Assistant,
		ToolCalls: []schema.ToolCall{{ID: "call-1", Function: schema.FunctionCall{Name: "echo_info", Arguments: `{}`}}},
	}})
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 || events[0].Type != domain.EventToolRequested {
		t.Fatalf("tool boundary events = %+v", events)
	}
	if m.hasPending || m.pendingText.Len() != 0 {
		t.Fatalf("already-durable tool preamble was not fenced: pending=%q", m.pendingText.String())
	}

	next, err := m.onMessageEvent(&adk.TypedMessageVariant[*schema.Message]{Message: &schema.Message{Role: schema.Assistant, Content: "final"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(next) != 2 || next[0].Type != domain.EventModelDelta || next[1].Type != domain.EventModelCompleted || strings.Contains(string(next[1].Payload), "before tool") {
		t.Fatalf("next round was contaminated: %+v", next)
	}
	if payloadDeltaOf(t, next[0].Payload) != "final" {
		t.Fatalf("next round delta = %q, want final", payloadDeltaOf(t, next[0].Payload))
	}
	completed := payloadCompletedOf(t, next[1].Payload)
	if completed.ContentSHA256 != sha256Hex([]byte("final")) || completed.ByteLen != len([]byte("final")) {
		t.Fatalf("next round completion = %+v", completed)
	}
}

func TestClampText(t *testing.T) {
	if got := clampText("hello", 1024); got != "hello" {
		t.Fatalf("small text must pass through, got %q", got)
	}
	long := strings.Repeat("x", 10000)
	got := clampText(long, 128)
	if len(got) >= len(long) {
		t.Fatal("clampText must shrink oversized input")
	}
}

func TestEventMapperClampsJSONEscapedToolResult(t *testing.T) {
	const budget = 4096
	m := newEventMapper("run-escaped", budget)
	result := strings.Repeat(`\"\\`, 8192)
	event := m.build(domain.EventToolFinished, payloadToolFinished{ToolCallID: "call", ToolName: tools.BashName, Result: result})
	if len(event.Payload) > budget {
		t.Fatalf("encoded payload = %d bytes, want <= %d", len(event.Payload), budget)
	}
}

func TestEventMapperBoundsCompleteToolFinishedPayload(t *testing.T) {
	const budget = 4096
	parts := []json.RawMessage{
		json.RawMessage(`{"type":"text","text":"` + strings.Repeat("a", 5000) + `"}`),
		json.RawMessage(`{"type":"text","text":"` + strings.Repeat("b", 5000) + `"}`),
	}
	event := newEventMapper("run-parts", budget).build(domain.EventToolFinished, payloadToolFinished{
		ToolCallID: "call", ToolName: "read_file", Result: "ok", Parts: parts,
	})
	if len(event.Payload) > budget {
		t.Fatalf("tool.finished payload = %d bytes, want <= %d", len(event.Payload), budget)
	}
	var payload payloadToolFinished
	if err := json.Unmarshal(event.Payload, &payload); err != nil {
		t.Fatal(err)
	}
	if len(payload.Parts) != 0 || !strings.Contains(payload.Result, "parts omitted") {
		t.Fatalf("oversized parts were not explicitly omitted: %+v", payload)
	}
}

func TestEventMapperBoundsApprovalReviewPayloadBeforeJournal(t *testing.T) {
	const budget = 8192
	m := newEventMapper("run-approval-bound", budget)
	event := m.build(domain.EventToolApprovalRequired, payloadToolApprovalRequired{
		ApprovalID: "approval-1", ToolCallID: "call-1", ToolName: "write_file", Action: "write_file",
		Args:    map[string]any{"content": strings.Repeat(`"\\`, 10000)},
		Preview: strings.Repeat("界", 100000), RiskFindings: []string{strings.Repeat("risk", 10000)},
	})
	if len(event.Payload) > budget {
		t.Fatalf("approval payload = %d bytes, want <= %d", len(event.Payload), budget)
	}
	var payload payloadToolApprovalRequired
	if err := json.Unmarshal(event.Payload, &payload); err != nil {
		t.Fatal(err)
	}
	if payload.ApprovalID != "approval-1" || payload.ToolCallID != "call-1" {
		t.Fatalf("approval identity was lost: %+v", payload)
	}
	if !strings.Contains(payload.Preview, "omitted") && !strings.Contains(payload.Preview, "truncated") {
		t.Fatalf("oversized preview was not marked: %q", payload.Preview)
	}
}

func TestBoundedApprovalReviewMatchesStoredAndJournalProjection(t *testing.T) {
	const budget = 64 << 10
	proposal := boundToolProposalReview(domain.ToolProposal{
		Action: "write_file", Target: "src/sk-live-abcdefghijkl/main.go", PreconditionHash: strings.Repeat("a", 64),
		Preview: "token sk-live-abcdefghijkl\n" + strings.Repeat("界", 50000), RiskFindings: []string{"api_key='private value' " + strings.Repeat("risk", 10000)},
	}, budget)
	if !strings.Contains(proposal.Target+proposal.Preview+strings.Join(proposal.RiskFindings, ""), "sk-live") || !strings.Contains(strings.Join(proposal.RiskFindings, ""), "private value") {
		t.Fatalf("stored approval review changed synthetic task data: %+v", proposal)
	}
	event := newEventMapper("run-review-parity", budget).build(domain.EventToolApprovalRequired, payloadToolApprovalRequired{
		ApprovalID: "approval-1", ToolCallID: "call-1", ToolName: "write_file", Args: map[string]any{"content": strings.Repeat("x", 100000)},
		Action: proposal.Action, Target: proposal.Target, PreconditionHash: proposal.PreconditionHash,
		Preview: proposal.Preview, RiskFindings: proposal.RiskFindings,
	})
	var journal payloadToolApprovalRequired
	if err := json.Unmarshal(event.Payload, &journal); err != nil {
		t.Fatal(err)
	}
	if journal.Preview != proposal.Preview || !reflect.DeepEqual(journal.RiskFindings, proposal.RiskFindings) {
		t.Fatalf("stored review and Journal diverged: stored preview=%d risks=%#v journal preview=%d risks=%#v", len(proposal.Preview), proposal.RiskFindings, len(journal.Preview), journal.RiskFindings)
	}
}

func TestEventMapperHonorsMinimumConfiguredPayloadBudget(t *testing.T) {
	const budget = 1024
	m := newEventMapper("run-min-budget", budget)
	events := []domain.RunEvent{
		m.build(domain.EventToolFinished, payloadToolFinished{ToolCallID: strings.Repeat("c", 500), ToolName: strings.Repeat("t", 5000), Result: strings.Repeat("r", 5000), Parts: []json.RawMessage{json.RawMessage(`{"text":"` + strings.Repeat("p", 5000) + `"}`)}}),
		m.build(domain.EventToolApprovalRequired, payloadToolApprovalRequired{ApprovalID: strings.Repeat("a", 5000), ToolCallID: strings.Repeat("c", 5000), ToolName: strings.Repeat("w", 5000), Face: strings.Repeat("f", 5000), Args: map[string]any{"secret": strings.Repeat("x", 5000)}, Preview: strings.Repeat("p", 5000), RiskFindings: []string{strings.Repeat("r", 5000)}}),
	}
	for _, event := range events {
		if len(event.Payload) > budget {
			t.Fatalf("%s payload = %d bytes, want <= %d: %s", event.Type, len(event.Payload), budget, event.Payload)
		}
	}
}

// capturingModel records the message list of every Stream call and
// answers a fixed reply, so tests can assert exactly what the engine fed
// the model (MA-1).
type capturingModel struct {
	mu     sync.Mutex
	inputs [][]domain.Message
}

func (c *capturingModel) Stream(_ context.Context, in []*domain.Message) (domain.Stream[*domain.Message], error) {
	c.mu.Lock()
	cp := make([]domain.Message, 0, len(in))
	for _, m := range in {
		cp = append(cp, *m)
	}
	c.inputs = append(c.inputs, cp)
	c.mu.Unlock()
	return &captureStream{chunks: []*domain.Message{{Role: domain.RoleAssistant, Content: "captured reply"}}}, nil
}

func (c *capturingModel) calls() [][]domain.Message {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make([][]domain.Message, len(c.inputs))
	copy(out, c.inputs)
	return out
}

// captureStream replays fixed reply chunks for the stream tests.
type captureStream struct {
	chunks []*domain.Message
	next   int
}

type gatedIncrementalModel struct {
	secondRecv chan struct{}
	release    chan struct{}
}

func (m *gatedIncrementalModel) Stream(context.Context, []*domain.Message) (domain.Stream[*domain.Message], error) {
	return &gatedIncrementalStream{secondRecv: m.secondRecv, release: m.release}, nil
}

type gatedIncrementalStream struct {
	next       int
	secondRecv chan struct{}
	release    chan struct{}
}

func (s *gatedIncrementalStream) Recv() (*domain.Message, error) {
	if s.next < 9 {
		content := "x"
		if s.next == 0 {
			content = "这"
		}
		s.next++
		return &domain.Message{Role: domain.RoleAssistant, Content: content}, nil
	}
	switch s.next {
	case 9:
		s.next++
		close(s.secondRecv)
		<-s.release
		return &domain.Message{Role: domain.RoleAssistant, Content: "是一句话"}, nil
	default:
		return nil, io.EOF
	}
}

func TestServicePersistsFirstModelChunkBeforeProviderEOF(t *testing.T) {
	model := &gatedIncrementalModel{secondRecv: make(chan struct{}), release: make(chan struct{})}
	released := false
	defer func() {
		if !released {
			close(model.release)
		}
	}()
	svc, backend, _ := newTestService(t, model)
	mustCreateSession(t, backend, "sess-incremental")
	runID, err := svc.Run(context.Background(), "sess-incremental", "stream")
	if err != nil {
		t.Fatal(err)
	}
	select {
	case <-model.secondRecv:
	case <-time.After(2 * time.Second):
		t.Fatal("provider never requested its second chunk")
	}
	seenFirst := false
	var events []domain.RunEvent
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) && !seenFirst {
		events = replayAll(t, backend, runID)
		for _, event := range events {
			if event.Type == domain.EventModelDelta && payloadDeltaOf(t, event.Payload) == "这" {
				seenFirst = true
			}
		}
		if !seenFirst {
			time.Sleep(5 * time.Millisecond)
		}
	}
	if !seenFirst {
		t.Fatalf("first chunk was not durable while provider remained open: %+v", events)
	}
	close(model.release)
	released = true
	waitForRunStatus(t, backend, runID, domain.RunCompleted)
	var got strings.Builder
	for _, event := range replayAll(t, backend, runID) {
		if event.Type == domain.EventModelDelta {
			got.WriteString(payloadDeltaOf(t, event.Payload))
		}
	}
	want := "这" + strings.Repeat("x", 8) + "是一句话"
	if got.String() != want {
		t.Fatalf("durable deltas = %q, want exactly-once provider text %q", got.String(), want)
	}
}

func (s *captureStream) Recv() (*domain.Message, error) {
	if s.next >= len(s.chunks) {
		return nil, io.EOF
	}
	chunk := s.chunks[s.next]
	s.next++
	return chunk, nil
}

// userAssistantPairs strips the leading run context: the static
// instruction and the per-run preamble both cross the adapter boundary
// as system messages, which the three-role domain vocabulary collapses
// to the assistant role, so every assistant entry before the first user
// message is leading context, not transcript.
func userAssistantPairs(msgs []domain.Message) [][2]string {
	var out [][2]string
	for _, m := range msgs {
		switch m.Role {
		case domain.RoleUser, domain.RoleAssistant:
			// Eino's official dynamic-tool middleware inserts a transient
			// user-role reminder listing deferred tools. It is model context,
			// not session transcript, so omit it from history assertions.
			if strings.HasPrefix(m.Content, "<available-deferred-tools>") {
				continue
			}
			out = append(out, [2]string{string(m.Role), m.Content})
		}
	}
	start := 0
	for start < len(out) && out[start][0] == string(domain.RoleAssistant) {
		start++
	}
	return out[start:]
}

// The second run of a session must carry the first turn's transcript:
// without the feed every turn is stateless (docs/v1-minimal-agent-proposal.md §1).
func TestServiceFeedsSessionHistory(t *testing.T) {
	cm := &capturingModel{}
	svc, backend, _ := newTestService(t, cm)
	mustCreateSession(t, backend, "sess-h")

	run1, err := svc.Run(context.Background(), "sess-h", "remember the code word bluebird")
	if err != nil {
		t.Fatalf("run 1: %v", err)
	}
	waitForRunStatus(t, backend, run1, domain.RunCompleted)

	run2, err := svc.Run(context.Background(), "sess-h", "what is the code word?")
	if err != nil {
		t.Fatalf("run 2: %v", err)
	}
	waitForRunStatus(t, backend, run2, domain.RunCompleted)

	calls := cm.calls()
	if len(calls) != 2 {
		t.Fatalf("model calls = %d, want 2", len(calls))
	}
	first := userAssistantPairs(calls[0])
	wantFirst := [][2]string{{"user", "remember the code word bluebird"}}
	if len(first) != len(wantFirst) || first[0] != wantFirst[0] {
		t.Fatalf("first run feed = %v, want %v", first, wantFirst)
	}
	second := userAssistantPairs(calls[1])
	wantSecond := [][2]string{
		{"user", "remember the code word bluebird"},
		{"assistant", "captured reply"},
		{"user", "what is the code word?"},
	}
	if len(second) != len(wantSecond) {
		t.Fatalf("second run feed = %v, want %v", second, wantSecond)
	}
	for i := range wantSecond {
		if second[i] != wantSecond[i] {
			t.Fatalf("second run feed[%d] = %v, want %v", i, second[i], wantSecond[i])
		}
	}
}

// History feeds are per-session: another session's transcript must never
// leak into the feed (multi-session isolation).
func TestServiceHistoryIsolatedAcrossSessions(t *testing.T) {
	cm := &capturingModel{}
	svc, backend, _ := newTestService(t, cm)
	mustCreateSession(t, backend, "sess-a")
	mustCreateSession(t, backend, "sess-b")

	run1, err := svc.Run(context.Background(), "sess-a", "a speaks first")
	if err != nil {
		t.Fatalf("run sess-a: %v", err)
	}
	waitForRunStatus(t, backend, run1, domain.RunCompleted)

	run2, err := svc.Run(context.Background(), "sess-b", "b speaks second")
	if err != nil {
		t.Fatalf("run sess-b: %v", err)
	}
	waitForRunStatus(t, backend, run2, domain.RunCompleted)

	calls := cm.calls()
	if len(calls) != 2 {
		t.Fatalf("model calls = %d, want 2", len(calls))
	}
	second := userAssistantPairs(calls[1])
	want := [][2]string{{"user", "b speaks second"}}
	if len(second) != len(want) || second[0] != want[0] {
		t.Fatalf("sess-b feed = %v, want exactly %v (no sess-a leakage)", second, want)
	}
}

// Every run's feed must be led by the per-run preamble (MA-2): persona and
// current date, ahead of any history. The official Eino tool-search
// middleware owns any deferred-tool discovery reminder separately.
func TestServiceRunLeadsWithPreamble(t *testing.T) {
	cm := &capturingModel{}
	svc, backend, _ := newTestService(t, cm)
	mustCreateSession(t, backend, "sess-p")

	runID, err := svc.Run(context.Background(), "sess-p", "echo hello")
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	waitForRunStatus(t, backend, runID, domain.RunCompleted)

	calls := cm.calls()
	if len(calls) != 1 {
		t.Fatalf("model calls = %d, want 1", len(calls))
	}
	feed := calls[0]
	if len(feed) < 2 {
		t.Fatalf("feed too short: %+v", feed)
	}
	// The adapter collapses system roles to assistant. The stable instruction
	// must precede the dynamic per-run preamble and the first user message.
	static := feed[0]
	if static.Role != domain.RoleAssistant || !strings.HasPrefix(static.Content, preamblePersona) {
		t.Fatalf("feed[0] = %+v, want stable instruction leading with %q", static, preamblePersona)
	}
	dynamic := feed[1]
	if !strings.Contains(dynamic.Content, "Today's date: ") {
		t.Fatalf("dynamic preamble missing date: %q", dynamic.Content)
	}
	if strings.Contains(static.Content, "echo_info") {
		t.Fatalf("static instruction must not contain request-scoped tool names: %q", static.Content)
	}
	if strings.Contains(dynamic.Content, "echo_info") || strings.Contains(dynamic.Content, "read-only; runs automatically") {
		t.Fatalf("dynamic preamble must not carry a full tool manifest: %q", dynamic.Content)
	}
	last := feed[len(feed)-1]
	if last.Role != domain.RoleUser || last.Content != "echo hello" {
		t.Fatalf("feed must end with the user message, got %+v", last)
	}
}

// assertNoNotebookMarker checks every message handed to the model across
// all recorded calls: no seeded notebook text may reach the model input.
// Service no longer accepts a notebook dependency at all, so there is no
// automatic read path left to count — marker absence is the behavioral
// evidence an ordinary turn never enumerates or injects notebook content.
func assertNoNotebookMarker(t *testing.T, calls [][]domain.Message, marker string) {
	t.Helper()
	var input strings.Builder
	for _, call := range calls {
		for i := range call {
			input.WriteString(call[i].Content)
			input.WriteByte('\n')
		}
	}
	if strings.Contains(input.String(), marker) {
		t.Fatalf("notebook content reached model input: %q", input.String())
	}
}

func TestServiceDoesNotInjectNotebook(t *testing.T) {
	ctx := context.Background()
	cm := &capturingModel{}

	backend, err := sqlite.Open(ctx, filepath.Join(t.TempDir(), "journal.db"))
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { _ = backend.Close() })

	ts, err := tools.Builtin(backend).Resolve([]string{tools.EchoInfoName})
	if err != nil {
		t.Fatalf("resolve tools: %v", err)
	}
	eng, err := NewEngine(ctx, WrapModel(cm), ts, EngineConfig{StreamBuffer: 8, MaxEventPayloadBytes: 64 << 10})
	if err != nil {
		t.Fatalf("new engine: %v", err)
	}
	svc := NewService(eng, "test", "test-model", ServiceDeps{
		Journal: backend, Runs: backend, Messages: backend, Sessions: backend, Sink: newTestSink(), Truncations: backend,
	})
	mustCreateSession(t, backend, "sess-nb")

	const marker = "bluebird-notebook-marker-7f3a"
	if err := backend.AppendNote(ctx, domain.Note{
		ID: "note_n0_marker", Content: marker + "\nsecond line", CreatedAt: time.Now().UnixMilli(),
	}); err != nil {
		t.Fatalf("seed note: %v", err)
	}

	// Ordinary turn: no automatic notebook enumeration or injection.
	runID, err := svc.Run(ctx, "sess-nb", "hello")
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	waitForRunStatus(t, backend, runID, domain.RunCompleted)
	assertNoNotebookMarker(t, cm.calls(), marker)

	// Continuation turn in the same session rebuilds history; the notebook
	// must stay outside it as well.
	runID, err = svc.Run(ctx, "sess-nb", "and again")
	if err != nil {
		t.Fatalf("continuation run: %v", err)
	}
	waitForRunStatus(t, backend, runID, domain.RunCompleted)
	assertNoNotebookMarker(t, cm.calls(), marker)

	// A reopened service on the same data (session resume after restart)
	// must not resume the read either.
	cm2 := &capturingModel{}
	eng2, err := NewEngine(ctx, WrapModel(cm2), ts, EngineConfig{StreamBuffer: 8, MaxEventPayloadBytes: 64 << 10})
	if err != nil {
		t.Fatalf("new engine: %v", err)
	}
	svc2 := NewService(eng2, "test", "test-model", ServiceDeps{
		Journal: backend, Runs: backend, Messages: backend, Sessions: backend, Sink: newTestSink(), Truncations: backend,
	})
	runID, err = svc2.Run(ctx, "sess-nb", "after restart")
	if err != nil {
		t.Fatalf("resumed-session run: %v", err)
	}
	waitForRunStatus(t, backend, runID, domain.RunCompleted)
	assertNoNotebookMarker(t, cm2.calls(), marker)
}

// payload decode helpers keep the tests readable.

func payloadDeltaOf(t *testing.T, b []byte) string {
	t.Helper()
	var p payloadModelDelta
	mustUnmarshal(t, b, &p)
	return p.Delta
}

func payloadCompletedOf(t *testing.T, b []byte) payloadModelCompletedV2 {
	t.Helper()
	var p payloadModelCompletedV2
	mustUnmarshal(t, b, &p)
	return p
}

func payloadReasonOf(t *testing.T, b []byte) string {
	t.Helper()
	var p payloadRunCancelled
	mustUnmarshal(t, b, &p)
	return p.Reason
}

func payloadFailureOf(t *testing.T, b []byte) (string, string) {
	t.Helper()
	var p payloadRunFailed
	mustUnmarshal(t, b, &p)
	return p.CauseCategory, p.Message
}

func mustUnmarshal(t *testing.T, b []byte, v any) {
	t.Helper()
	if err := json.Unmarshal(b, v); err != nil {
		t.Fatalf("decode payload %s: %v", b, err)
	}
}

type recordingChatModel struct {
	inner  model.ToolCallingChatModel
	mu     sync.Mutex
	inputs [][]*schema.Message
}

func (m *recordingChatModel) Generate(ctx context.Context, input []*schema.Message, opts ...model.Option) (*schema.Message, error) {
	m.record(input)
	return m.inner.Generate(ctx, input, opts...)
}

func (m *recordingChatModel) Stream(ctx context.Context, input []*schema.Message, opts ...model.Option) (*schema.StreamReader[*schema.Message], error) {
	m.record(input)
	return m.inner.Stream(ctx, input, opts...)
}

func (m *recordingChatModel) WithTools(tools []*schema.ToolInfo) (model.ToolCallingChatModel, error) {
	next, err := m.inner.WithTools(tools)
	if err != nil {
		return nil, err
	}
	m.inner = next
	return m, nil
}

func (m *recordingChatModel) record(input []*schema.Message) {
	m.mu.Lock()
	defer m.mu.Unlock()
	cp := make([]*schema.Message, 0, len(input))
	for _, msg := range input {
		if msg == nil {
			continue
		}
		clone := *msg
		if len(msg.ToolCalls) > 0 {
			clone.ToolCalls = append([]schema.ToolCall(nil), msg.ToolCalls...)
		}
		cp = append(cp, &clone)
	}
	m.inputs = append(m.inputs, cp)
}

func (m *recordingChatModel) lastInput() []*schema.Message {
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(m.inputs) == 0 {
		return nil
	}
	return m.inputs[len(m.inputs)-1]
}

func TestServiceFeedsToolTraceAndRequestDigest(t *testing.T) {
	ctx := context.Background()
	backend, err := sqlite.Open(ctx, filepath.Join(t.TempDir(), "tool-feed.db"))
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { _ = backend.Close() })

	recorder := &recordingChatModel{inner: NewScriptedModel(
		schema.AssistantMessage("", []schema.ToolCall{{
			ID:       "call-echo-1",
			Function: schema.FunctionCall{Name: tools.EchoInfoName, Arguments: `{"text":"hi"}`},
		}}),
		schema.AssistantMessage("echoed", nil),
		schema.AssistantMessage("second turn", nil),
	)}
	ts, err := tools.Builtin(backend).Resolve([]string{tools.EchoInfoName})
	if err != nil {
		t.Fatalf("resolve tools: %v", err)
	}
	eng, err := NewEngine(ctx, recorder, ts, EngineConfig{StreamBuffer: 8, MaxEventPayloadBytes: 64 << 10})
	if err != nil {
		t.Fatalf("new engine: %v", err)
	}
	svc := NewService(eng, "scripted", "scripted-v0", ServiceDeps{
		Journal: backend, Runs: backend, Messages: backend, Sink: newTestSink(),
	})
	mustCreateSession(t, backend, "sess-tools")

	run1, err := svc.Run(ctx, "sess-tools", "please echo")
	if err != nil {
		t.Fatalf("run 1: %v", err)
	}
	waitForRunStatus(t, backend, run1, domain.RunCompleted)

	stored, err := backend.ListMessages(ctx, "sess-tools")
	if err != nil {
		t.Fatalf("list messages: %v", err)
	}
	var sawCall, sawResult bool
	for _, msg := range stored {
		if msg.Role == domain.RoleAssistant && msg.ToolCallID == "call-echo-1" {
			sawCall = true
		}
		if msg.Role == domain.RoleTool && msg.ToolCallID == "call-echo-1" {
			sawResult = true
		}
	}
	if !sawCall || !sawResult {
		t.Fatalf("message projection missing tool turn: %+v", stored)
	}

	run2, err := svc.Run(ctx, "sess-tools", "what did you echo?")
	if err != nil {
		t.Fatalf("run 2: %v", err)
	}
	waitForRunStatus(t, backend, run2, domain.RunCompleted)

	feed := recorder.lastInput()
	var sawToolRole bool
	for _, msg := range feed {
		if msg.Role == schema.Tool && msg.ToolCallID == "call-echo-1" {
			sawToolRole = true
		}
	}
	if !sawToolRole {
		t.Fatalf("second-run feed missing tool result: %+v", feed)
	}

	if _, _, err := buildRunContext(ContextPolicy{}, "unused-preamble", stored, "what did you echo?"); err != nil {
		t.Fatalf("rebuild context: %v", err)
	}

	events := replayAll(t, backend, run2)
	var req payloadModelRequest
	found := false
	for _, ev := range events {
		if ev.Type == domain.EventModelRequest {
			mustUnmarshal(t, ev.Payload, &req)
			found = true
			break
		}
	}
	if !found {
		t.Fatal("second run missing model.request")
	}
	// The v3 request digests the exact invocation input the model received
	// (feed), which includes the injected preamble/history rows the
	// surrogate request never saw. rebuilt is still asserted above for the
	// feed itself; here the digest must equal the actual feed digest.
	gotBody := stripSystemRequestRows(req.Messages)
	wantBody := stripSystemRequestRows(digestModelRequest(feed, nil).Messages)
	if len(gotBody) != len(wantBody) {
		t.Fatalf("model.request body = %+v, want %+v", gotBody, wantBody)
	}
	for i := range wantBody {
		if gotBody[i].Role != wantBody[i].Role || gotBody[i].ContentSHA256 != wantBody[i].ContentSHA256 || gotBody[i].ToolCallID != wantBody[i].ToolCallID {
			t.Fatalf("model.request body[%d] = %+v, want %+v", i, gotBody[i], wantBody[i])
		}
	}
}

func stripSystemRequestRows(in []payloadModelRequestMessage) []payloadModelRequestMessage {
	out := make([]payloadModelRequestMessage, 0, len(in))
	for _, row := range in {
		if row.Role == "system" {
			continue
		}
		out = append(out, row)
	}
	return out
}

// Streaming chunk events must not consume the run events budget: one mapped
// event per streamed chunk makes any substantive reply exceed MaxEvents on
// its own (TT-4). Semantic events keep charging, and the model-call /
// tool-call budgets remain the runaway guard.
func TestReserveMappedBudgetSkipsStreamingDeltas(t *testing.T) {
	ledger, err := NewBudgetLedger(BudgetPolicy{MaxEvents: 5, MaxModelCalls: 5, MaxToolCalls: 5, MaxRetries: 5})
	if err != nil {
		t.Fatal(err)
	}
	m := newEventMapper("run-budget", 0)
	deltas := make([]domain.RunEvent, 0, 600)
	for i := 0; i < 600; i++ {
		deltas = append(deltas, domain.RunEvent{Type: domain.EventModelDelta})
	}
	if err := m.reserveMappedBudget(ledger, deltas); err != nil {
		t.Fatalf("600 streamed deltas must not consume the events budget: %v", err)
	}
	if err := m.reserveMappedBudget(ledger, []domain.RunEvent{{Type: domain.EventModelCompleted}}); err != nil {
		t.Fatalf("semantic events keep charging: %v", err)
	}
	semantic := make([]domain.RunEvent, 0, 6)
	for i := 0; i < 6; i++ {
		semantic = append(semantic, domain.RunEvent{Type: domain.EventModelUsage})
	}
	if err := m.reserveMappedBudget(ledger, semantic); !errors.Is(err, ErrBudgetExceeded) {
		t.Fatalf("events budget must still trip on semantic events, got: %v", err)
	}
}

// ---------------------------------------------------------------------
// Atomic continuity admission (SC-D4 §7): one transaction commits the
// user message, the active run row, the ordered startup events and the
// dedup receipt. Retries replay the committed identity, a changed payload
// under the same request_id is a conflict, and the just-allocated private
// workspace rolls back on a failed admission.

type failingContinuityStore struct {
	storage.ContinuityStore
	err error
}

func (f failingContinuityStore) CommitContinuityRun(context.Context, storage.ContinuityAdmission) (storage.ContinuityResult, error) {
	return storage.ContinuityResult{}, f.err
}

func newContinuityService(t *testing.T, chatModel domain.ChatModel) (*Service, *sqlite.Backend, *testSink) {
	t.Helper()
	svc, backend, sink := newTestService(t, chatModel)
	svc.deps.Continuity = backend
	return svc, backend, sink
}

func continuityWorkspaceEntries(t *testing.T, root string) int {
	t.Helper()
	entries, err := os.ReadDir(root)
	if errors.Is(err, os.ErrNotExist) {
		return 0
	}
	if err != nil {
		t.Fatalf("read workspace root: %v", err)
	}
	return len(entries)
}

func TestContinuityAtomic(t *testing.T) {
	svc, backend, sink := newContinuityService(t, testsupport.NewEchoModel())
	ctx := context.Background()
	sessionID := domain.SessionID("cont-atomic")
	mustCreateSession(t, backend, sessionID)
	t.Cleanup(func() { svc.CancelAll(); svc.WaitIdle(context.Background()) })

	input := &domain.ContinuityInput{RequestID: "req-atomic-1"}
	runID, err := svc.RunWithOptions(ctx, sessionID, "hello continuity", RunOptions{Continuity: input})
	if err != nil {
		t.Fatalf("continuity run: %v", err)
	}
	waitForRunStatus(t, backend, runID, domain.RunCompleted)
	svc.WaitIdle(ctx)

	receipt, found, err := backend.FindContinuityReceipt(ctx, sessionID, storage.ContinuityOperationAdmission, input.RequestID)
	if err != nil || !found {
		t.Fatalf("admission receipt found=%v err=%v", found, err)
	}
	if receipt.RunID != runID {
		t.Fatalf("receipt run = %s, want %s", receipt.RunID, runID)
	}
	if receipt.EventSeq != 1 {
		t.Fatalf("receipt event_seq = %d, want the committed startup tail 1", receipt.EventSeq)
	}

	events := replayAll(t, backend, runID)
	if len(events) == 0 || events[0].Type != domain.EventRunStarted {
		t.Fatalf("first journaled event type = %v", events[0].Type)
	}
	var started payloadRunStarted
	mustUnmarshal(t, events[0].Payload, &started)
	if started.HistoryScope == nil || started.HistoryScope.DestinationSessionID != sessionID {
		t.Fatalf("run.started history_scope = %+v, want destination %s", started.HistoryScope, sessionID)
	}
	published := sink.snapshot()
	if len(published) == 0 || published[0].Type != domain.EventRunStarted {
		t.Fatal("committed run.started was not published after the atomic commit")
	}
	messages, err := backend.ListMessages(ctx, sessionID)
	if err != nil {
		t.Fatalf("list messages: %v", err)
	}
	if len(messages) == 0 || messages[0].Role != domain.RoleUser || messages[0].Content != "hello continuity" {
		t.Fatalf("admitted user message = %v", messages)
	}
}

func TestContinuityRetry(t *testing.T) {
	svc, backend, _ := newContinuityService(t, testsupport.NewEchoModel())
	ctx := context.Background()
	sessionID := domain.SessionID("cont-retry")
	mustCreateSession(t, backend, sessionID)
	t.Cleanup(func() { svc.CancelAll(); svc.WaitIdle(context.Background()) })

	input := &domain.ContinuityInput{RequestID: "req-retry-1"}
	first, err := svc.RunWithOptions(ctx, sessionID, "ship it", RunOptions{Continuity: input})
	if err != nil {
		t.Fatalf("first run: %v", err)
	}
	waitForRunStatus(t, backend, first, domain.RunCompleted)
	svc.WaitIdle(ctx)

	second, err := svc.RunWithOptions(ctx, sessionID, "ship it", RunOptions{Continuity: input})
	if err != nil {
		t.Fatalf("identical retry: %v", err)
	}
	if second != first {
		t.Fatalf("identical retry admitted run %s, want original %s", second, first)
	}
	// The lost-response contract holds even after the run finished.
	third, err := svc.RunWithOptions(ctx, sessionID, "ship it", RunOptions{Continuity: input})
	if err != nil {
		t.Fatalf("post-terminal retry: %v", err)
	}
	if third != first {
		t.Fatalf("post-terminal retry admitted run %s, want original %s", third, first)
	}

	messages, err := backend.ListMessages(ctx, sessionID)
	if err != nil {
		t.Fatalf("list messages: %v", err)
	}
	userMessages := 0
	for _, message := range messages {
		if message.Role == domain.RoleUser {
			userMessages++
		}
	}
	if userMessages != 1 {
		t.Fatalf("retries admitted %d user messages, want 1", userMessages)
	}
	startedCount := 0
	for _, ev := range replayAll(t, backend, first) {
		if ev.Type == domain.EventRunStarted {
			startedCount++
		}
	}
	if startedCount != 1 {
		t.Fatalf("run has %d run.started events, want 1", startedCount)
	}

	if _, err := svc.RunWithOptions(ctx, sessionID, "different text", RunOptions{Continuity: input}); !errors.Is(err, storage.ErrConflict) {
		t.Fatalf("changed payload under same request_id = %v, want conflict", err)
	}
}

func TestContinuityUnavailableFailsClosed(t *testing.T) {
	svc, backend, _ := newTestService(t, testsupport.NewEchoModel())
	ctx := context.Background()
	sessionID := domain.SessionID("cont-unavailable")
	mustCreateSession(t, backend, sessionID)
	_, err := svc.RunWithOptions(ctx, sessionID, "hi", RunOptions{Continuity: &domain.ContinuityInput{RequestID: "r-1"}})
	if !errors.Is(err, ErrContinuityUnavailable) {
		t.Fatalf("continuity submission without atomic backend = %v, want ErrContinuityUnavailable", err)
	}
}

func TestContinuityAdmissionWorkspaceRollback(t *testing.T) {
	svc, backend, _ := newContinuityService(t, testsupport.NewEchoModel())
	ctx := context.Background()
	sessionID := domain.SessionID("cont-rollback")
	mustCreateSession(t, backend, sessionID)
	root := filepath.Join(t.TempDir(), "workspaces")
	manager, err := NewSessionWorkspaceManager(root, backend, backend)
	if err != nil {
		t.Fatalf("workspace manager: %v", err)
	}
	svc.deps.Workspaces = manager
	input := &domain.ContinuityInput{RequestID: "req-rollback-1"}

	// A definite pre-commit failure reaps the freshly created private dir.
	svc.deps.Continuity = failingContinuityStore{ContinuityStore: backend, err: errors.New("injected pre-commit failure")}
	if _, err := svc.RunWithOptions(ctx, sessionID, "task", RunOptions{Continuity: input}); err == nil {
		t.Fatal("injected failure must surface")
	}
	if got := continuityWorkspaceEntries(t, root); got != 0 {
		t.Fatalf("failed admission left %d workspace entries, want 0", got)
	}

	// An uncertain commit outcome preserves the dir: the rows may already
	// be durable and the engine may be writing into it.
	svc.deps.Continuity = failingContinuityStore{ContinuityStore: backend, err: storage.ErrCommitUncertain}
	if _, err := svc.RunWithOptions(ctx, sessionID, "task", RunOptions{Continuity: input}); !errors.Is(err, storage.ErrCommitUncertain) {
		t.Fatalf("uncertain commit = %v, want ErrCommitUncertain", err)
	}
	if got := continuityWorkspaceEntries(t, root); got != 1 {
		t.Fatalf("uncertain admission left %d workspace entries, want preserved 1", got)
	}
}
