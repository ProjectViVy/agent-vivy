package runtime

// VCP-B1 tests: dual-track queue, boundary-safe steering (Eino capability
// check → mechanism M2: WithCancel + CancelAfterToolCalls/ChatModel +
// checkpoint resume + ChatModelAgentResumeData.HistoryModifier), journal
// durability, lane modes.

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/storage/sqlite"
	"agent-vivy/internal/testsupport"
	"agent-vivy/internal/tools"
)

// gateModel blocks its FIRST Stream call until released (or ctx cancelled),
// then answers instantly. It records every request's message list so tests
// can assert what a resume leg injected into model-visible history.
type gateModel struct {
	entered chan struct{}
	release chan struct{}

	mu     sync.Mutex
	inputs [][]*domain.Message
}

func newGateModel() *gateModel {
	return &gateModel{entered: make(chan struct{}), release: make(chan struct{})}
}

func (m *gateModel) Stream(ctx context.Context, input []*domain.Message) (domain.Stream[*domain.Message], error) {
	m.mu.Lock()
	m.inputs = append(m.inputs, input)
	call := len(m.inputs)
	m.mu.Unlock()
	if call == 1 {
		close(m.entered)
		select {
		case <-m.release:
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	return &gateStream{message: &domain.Message{Role: domain.RoleAssistant, Content: "gate reply"}}, nil
}

func (m *gateModel) lastInputUserTexts() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	last := m.inputs[len(m.inputs)-1]
	var out []string
	for _, msg := range last {
		if msg != nil && msg.Role == domain.RoleUser {
			out = append(out, msg.Content)
		}
	}
	return out
}

type gateStream struct {
	message *domain.Message
	done    bool
}

func (s *gateStream) Recv() (*domain.Message, error) {
	if s.done {
		return nil, io.EOF
	}
	s.done = true
	return s.message, nil
}

var _ domain.ChatModel = (*gateModel)(nil)
var _ domain.Stream[*domain.Message] = (*gateStream)(nil)

// newQueueTestService wires checkpoints — the steer resume leg needs the
// two-layer checkpoint bridge, unlike plain newTestService.
func newQueueTestService(t *testing.T, model domain.ChatModel) (*Service, *sqlite.Backend) {
	t.Helper()
	ctx := context.Background()
	backend, err := sqlite.Open(ctx, filepath.Join(t.TempDir(), "queue.db"))
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { _ = backend.Close() })
	ts, err := tools.Builtin(backend).Resolve([]string{tools.EchoInfoName})
	if err != nil {
		t.Fatalf("resolve tools: %v", err)
	}
	checkpoints, err := NewVersionedCheckpointStore(backend.Blobs(), "test")
	if err != nil {
		t.Fatalf("checkpoint store: %v", err)
	}
	eng, err := NewEngine(ctx, WrapModel(model), ts, EngineConfig{
		StreamBuffer: 8, MaxEventPayloadBytes: 64 << 10, Checkpoints: checkpoints,
	})
	if err != nil {
		t.Fatalf("new engine: %v", err)
	}
	svc := NewService(eng, "test", "test-model", ServiceDeps{
		Journal: backend, Runs: backend, Messages: backend, Notes: backend,
		Sessions: backend, Sink: newTestSink(), Truncations: backend,
	})
	return svc, backend
}

func waitFor(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %s", what)
}

func runTerminal(t *testing.T, backend *sqlite.Backend, runID domain.RunID) domain.RunStatus {
	t.Helper()
	run, err := backend.GetRun(context.Background(), runID)
	if err != nil {
		t.Fatalf("get run %s: %v", runID, err)
	}
	return run.Status
}

func journalEvents(t *testing.T, backend *sqlite.Backend, runID domain.RunID) []domain.RunEvent {
	t.Helper()
	it, err := backend.Replay(context.Background(), runID, 0)
	if err != nil {
		t.Fatalf("replay %s: %v", runID, err)
	}
	defer func() { _ = it.Close() }()
	var out []domain.RunEvent
	for it.Next() {
		out = append(out, it.Value().Event)
	}
	return out
}

// scriptedToolModel drives a multi-turn run directly at the eino schema
// seam (the domain ChatModel adapter cannot express tool calls). Call 1
// requests the echo_info tool; call 2 blocks until released, then requests
// it again — the pending boundary cancel lands at the post-tool safe point;
// call 3+ answers with plain text. Every call's input is recorded.
type scriptedToolModel struct {
	entered chan struct{} // closed when call 2 starts
	release chan struct{}

	mu     sync.Mutex
	inputs [][]*schema.Message
}

func newScriptedToolModel() *scriptedToolModel {
	return &scriptedToolModel{entered: make(chan struct{}), release: make(chan struct{})}
}

func toolCallMessage(id, name, args string) *schema.Message {
	return &schema.Message{
		Role: schema.Assistant,
		ToolCalls: []schema.ToolCall{{
			ID:       id,
			Type:     "function",
			Function: schema.FunctionCall{Name: name, Arguments: args},
		}},
	}
}

func oneShot(msg *schema.Message) *schema.StreamReader[*schema.Message] {
	r, w := schema.Pipe[*schema.Message](1)
	go func() {
		w.Send(msg, nil)
		w.Close()
	}()
	return r
}

func (m *scriptedToolModel) record(input []*schema.Message) int {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.inputs = append(m.inputs, input)
	return len(m.inputs)
}

func (m *scriptedToolModel) callCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.inputs)
}

func (m *scriptedToolModel) lastUserTexts() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	var out []string
	for _, msg := range m.inputs[len(m.inputs)-1] {
		if msg != nil && msg.Role == schema.User {
			out = append(out, msg.Content)
		}
	}
	return out
}

func (m *scriptedToolModel) Stream(ctx context.Context, input []*schema.Message, _ ...model.Option) (*schema.StreamReader[*schema.Message], error) {
	call := m.record(input)
	switch call {
	case 1:
		return oneShot(toolCallMessage("call-1", tools.EchoInfoName, `{"text":"one"}`)), nil
	case 2:
		close(m.entered)
		select {
		case <-m.release:
		case <-ctx.Done():
			return nil, ctx.Err()
		}
		return oneShot(toolCallMessage("call-2", tools.EchoInfoName, `{"text":"two"}`)), nil
	default:
		return oneShot(&schema.Message{Role: schema.Assistant, Content: "done"}), nil
	}
}

func (m *scriptedToolModel) Generate(ctx context.Context, input []*schema.Message, opts ...model.Option) (*schema.Message, error) {
	r, err := m.Stream(ctx, input, opts...)
	if err != nil {
		return nil, err
	}
	defer r.Close()
	msg, err := r.Recv()
	return msg, err
}

func (m *scriptedToolModel) WithTools(_ []*schema.ToolInfo) (model.ToolCallingChatModel, error) {
	return m, nil
}

var _ model.ToolCallingChatModel = (*scriptedToolModel)(nil)

// newScriptedService builds a service whose engine is bound to a
// schema-level model (no domain ChatModel wrap) — needed for tool-call
// flows that exercise turn-boundary steering.
func newScriptedService(t *testing.T, m model.ToolCallingChatModel) (*Service, *sqlite.Backend) {
	t.Helper()
	ctx := context.Background()
	backend, err := sqlite.Open(ctx, filepath.Join(t.TempDir(), "queue.db"))
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { _ = backend.Close() })
	ts, err := tools.Builtin(backend).Resolve([]string{tools.EchoInfoName})
	if err != nil {
		t.Fatalf("resolve tools: %v", err)
	}
	checkpoints, err := NewVersionedCheckpointStore(backend.Blobs(), "test")
	if err != nil {
		t.Fatalf("checkpoint store: %v", err)
	}
	eng, err := NewEngine(ctx, m, ts, EngineConfig{
		StreamBuffer: 8, MaxEventPayloadBytes: 64 << 10, Checkpoints: checkpoints,
	})
	if err != nil {
		t.Fatalf("new engine: %v", err)
	}
	svc := NewService(eng, "test", "test-model", ServiceDeps{
		Journal: backend, Runs: backend, Messages: backend, Notes: backend,
		Sessions: backend, Sink: newTestSink(), Truncations: backend,
	})
	return svc, backend
}

func TestSteerInjectsAtTurnBoundary(t *testing.T) {
	stm := newScriptedToolModel()
	svc, backend := newScriptedService(t, stm)
	ctx := context.Background()
	mustCreateSession(t, backend, "sess-steer")

	runDone := make(chan error, 1)
	var runID domain.RunID
	go func() {
		var err error
		runID, err = svc.Run(ctx, "sess-steer", "initial prompt")
		runDone <- err
	}()
	<-stm.entered // call 1's tool batch done; call 2 in flight (blocked)

	item, err := svc.Steer(ctx, "sess-steer", "steer while running")
	if err != nil {
		t.Fatalf("steer: %v", err)
	}
	if item.Track != domain.QueueTrackSteer {
		t.Fatalf("track = %q, want steer", item.Track)
	}
	close(stm.release)

	if err := <-runDone; err != nil {
		t.Fatalf("run: %v", err)
	}
	waitFor(t, "resume leg model call", func() bool {
		return stm.callCount() >= 3
	})
	// The resume leg's HistoryModifier appends the steered user message to
	// model-visible history inside the SAME run — mid-run injection, not a
	// fresh run's prompt.
	found := false
	for _, text := range stm.lastUserTexts() {
		if text == "steer while running" {
			found = true
		}
	}
	if !found {
		t.Fatalf("steer text absent from resumed model history: %v", stm.lastUserTexts())
	}
	waitFor(t, "terminal status", func() bool {
		return runTerminal(t, backend, runID) == domain.RunCompleted
	})
	// Journal carries the single continuity marker.
	steered := false
	for _, ev := range journalEvents(t, backend, runID) {
		if ev.Type == domain.EventTurnSteered {
			steered = true
		}
	}
	if !steered {
		t.Fatal("turn.steered marker missing from run journal")
	}
	// And no extra run was admitted: steering stayed inside this run.
	runs, err := backend.ListRunsBySession(ctx, "sess-steer")
	if err != nil || len(runs) != 1 {
		t.Fatalf("runs = %v %v — steer must not spawn a follow-up run", runs, err)
	}
}

func TestSteerOnIdleSessionIsUnavailable(t *testing.T) {
	svc, backend := newQueueTestService(t, testsupport.NewEchoModel())
	mustCreateSession(t, backend, "sess-idle")
	_, err := svc.Steer(context.Background(), "sess-idle", "nothing running")
	if !errors.Is(err, ErrQueueUnavailable) {
		t.Fatalf("steer on idle session = %v, want ErrQueueUnavailable", err)
	}
}

func TestFollowUpAdmitsAfterSettle(t *testing.T) {
	model := newGateModel()
	svc, backend := newQueueTestService(t, model)
	ctx := context.Background()
	mustCreateSession(t, backend, "sess-follow")

	runDone := make(chan error, 1)
	go func() {
		_, err := svc.Run(ctx, "sess-follow", "first")
		runDone <- err
	}()
	<-model.entered
	item, err := svc.FollowUp(ctx, "sess-follow", "queued follow-up")
	if err != nil {
		t.Fatalf("follow_up: %v", err)
	}
	if item.Track != domain.QueueTrackFollowUp {
		t.Fatalf("track = %q, want follow_up", item.Track)
	}
	close(model.release)
	if err := <-runDone; err != nil {
		t.Fatalf("run: %v", err)
	}
	// The settle drain starts a second run automatically.
	waitFor(t, "auto-admitted run", func() bool {
		runs, err := backend.ListRunsBySession(ctx, "sess-follow")
		return err == nil && len(runs) == 2
	})
	runs, _ := backend.ListRunsBySession(ctx, "sess-follow")
	waitFor(t, "second run terminal", func() bool {
		return runTerminal(t, backend, runs[1].ID) == domain.RunCompleted
	})
	// Admission is journaled on the NEW run (turn.dequeued reason=started).
	dequeued := false
	for _, ev := range journalEvents(t, backend, runs[1].ID) {
		if ev.Type == domain.EventTurnDequeued {
			dequeued = true
		}
	}
	if !dequeued {
		t.Fatal("turn.dequeued marker missing on admitted run journal")
	}
	if got := svc.QueueState(ctx, "sess-follow", ""); len(got.FollowUps)+len(got.Steering) != 0 {
		t.Fatalf("queue not drained: %+v", got)
	}
}

func TestFollowUpOneAtATimeKeepsTail(t *testing.T) {
	model := newGateModel()
	svc, backend := newQueueTestService(t, model)
	ctx := context.Background()
	mustCreateSession(t, backend, "sess-one")
	if err := svc.SetQueueMode(ctx, "sess-one", domain.QueueTrackFollowUp, domain.QueueModeOneAtATime); err != nil {
		t.Fatalf("set mode: %v", err)
	}
	runDone := make(chan error, 1)
	go func() {
		_, err := svc.Run(ctx, "sess-one", "first")
		runDone <- err
	}()
	<-model.entered
	if _, err := svc.FollowUp(ctx, "sess-one", "fu-1"); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.FollowUp(ctx, "sess-one", "fu-2"); err != nil {
		t.Fatal(err)
	}
	close(model.release)
	<-runDone
	// one-at-a-time admits exactly one queued item per settle. Each
	// admission cascades into its own run, so the observable end state is
	// three runs — not the transient count of two.
	waitFor(t, "cascade admitted", func() bool {
		runs, _ := backend.ListRunsBySession(ctx, "sess-one")
		return len(runs) == 3
	})
	runs, _ := backend.ListRunsBySession(ctx, "sess-one")
	waitFor(t, "admitted runs terminal", func() bool {
		return runTerminal(t, backend, runs[1].ID) == domain.RunCompleted &&
			runTerminal(t, backend, runs[2].ID) == domain.RunCompleted
	})
	// Each settle admitted exactly one item: run 2 carries fu-1's dequeue,
	// run 3 carries fu-2's — never both on one run (that would be `all`).
	deq := map[string]bool{}
	for _, ev := range journalEvents(t, backend, runs[1].ID) {
		if ev.Type == domain.EventTurnDequeued {
			deq["r2"] = true
		}
	}
	for _, ev := range journalEvents(t, backend, runs[2].ID) {
		if ev.Type == domain.EventTurnDequeued {
			deq["r3"] = true
		}
	}
	if !deq["r2"] || !deq["r3"] {
		t.Fatalf("expected one dequeue marker on each admitted run journal, got %v", deq)
	}
}

func TestQueueRebuildsFromNewestRunJournal(t *testing.T) {
	model := newGateModel()
	svc, backend := newQueueTestService(t, model)
	ctx := context.Background()
	mustCreateSession(t, backend, "sess-rebuild")

	runDone := make(chan error, 1)
	go func() {
		_, err := svc.Run(ctx, "sess-rebuild", "first")
		runDone <- err
	}()
	<-model.entered
	if _, err := svc.FollowUp(ctx, "sess-rebuild", "survives restart"); err != nil {
		t.Fatal(err)
	}
	// Block the admitted run on a second gate so the queue tail (none) isn't
	// the check — instead assert the queued marker is on run 1's journal and
	// replay recovers it in a FRESH service over the same backend.
	svc2 := NewService(svc.engine, "test", "test-model", ServiceDeps{
		Journal: backend, Runs: backend, Messages: backend, Notes: backend,
		Sessions: backend, Sink: newTestSink(), Truncations: backend,
	})
	state := svc2.QueueState(ctx, "sess-rebuild", "")
	if len(state.FollowUps) != 1 || state.FollowUps[0].Text != "survives restart" {
		t.Fatalf("rebuilt queue = %+v", state)
	}
	close(model.release)
	<-runDone
	svc.CancelAll()
	svc.WaitIdle(ctx)
}

func TestClearQueueReturnsTexts(t *testing.T) {
	model := newGateModel()
	svc, backend := newQueueTestService(t, model)
	ctx := context.Background()
	mustCreateSession(t, backend, "sess-clear")
	runDone := make(chan error, 1)
	go func() {
		_, err := svc.Run(ctx, "sess-clear", "first")
		runDone <- err
	}()
	<-model.entered
	if _, err := svc.FollowUp(ctx, "sess-clear", "a"); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.FollowUp(ctx, "sess-clear", "b"); err != nil {
		t.Fatal(err)
	}
	state := svc.ClearQueue(ctx, "sess-clear")
	if len(state.FollowUps) != 2 {
		t.Fatalf("cleared = %+v", state)
	}
	if got := svc.QueueState(ctx, "sess-clear", ""); len(got.FollowUps) != 0 {
		t.Fatalf("queue not empty after clear: %+v", got)
	}
	close(model.release)
	<-runDone
	svc.CancelAll()
	svc.WaitIdle(ctx)
}

func TestSteerDuringSuspendedRunDemotesToFollowUp(t *testing.T) {
	svc, backend := newQueueTestService(t, testsupport.NewEchoModel())
	ctx := context.Background()
	mustCreateSession(t, backend, "sess-susp")
	// A suspended (approval/question) run cannot take a steer — steering
	// mid-tool would violate the boundary rule, so the item demotes.
	svc.mu.Lock()
	svc.runSessions["run-susp"] = "sess-susp"
	svc.active["run-susp"] = func() {}
	svc.pending["run-susp"] = pendingRun{}
	svc.mu.Unlock()
	item, err := svc.Steer(ctx, "sess-susp", "steer while suspended")
	if err != nil {
		t.Fatalf("steer: %v", err)
	}
	if item.Track != domain.QueueTrackFollowUp {
		t.Fatalf("track = %q, want follow_up demotion", item.Track)
	}
	state := svc.QueueState(ctx, "sess-susp", "")
	if len(state.FollowUps) != 1 || len(state.Steering) != 0 {
		t.Fatalf("queue = %+v", state)
	}
}

func TestDequeuePopsNewestFollowUp(t *testing.T) {
	model := newGateModel()
	svc, backend := newQueueTestService(t, model)
	ctx := context.Background()
	mustCreateSession(t, backend, "sess-deq")

	runDone := make(chan error, 1)
	go func() {
		_, err := svc.Run(ctx, "sess-deq", "first")
		runDone <- err
	}()
	<-model.entered
	if _, err := svc.FollowUp(ctx, "sess-deq", "older"); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.FollowUp(ctx, "sess-deq", "newest"); err != nil {
		t.Fatal(err)
	}
	item, ok := svc.Dequeue(ctx, "sess-deq")
	if !ok || item.Text != "newest" {
		t.Fatalf("dequeue = %+v ok=%v, want newest", item, ok)
	}
	state := svc.QueueState(ctx, "sess-deq", "")
	if len(state.FollowUps) != 1 || state.FollowUps[0].Text != "older" {
		t.Fatalf("tail after dequeue = %+v", state.FollowUps)
	}
	// Second dequeue drains the lane; an empty lane reports not-found.
	if item, ok := svc.Dequeue(ctx, "sess-deq"); !ok || item.Text != "older" {
		t.Fatalf("second dequeue = %+v ok=%v", item, ok)
	}
	if _, ok := svc.Dequeue(ctx, "sess-deq"); ok {
		t.Fatal("empty lane must report not-found")
	}
	// Journal carries turn.dequeued{reason:"dequeued", text} per item.
	var reasons []string
	runs, _ := backend.ListRunsBySession(ctx, "sess-deq")
	for _, ev := range journalEvents(t, backend, runs[len(runs)-1].ID) {
		if ev.Type != domain.EventTurnDequeued {
			continue
		}
		var p payloadTurnDequeued
		if err := json.Unmarshal(ev.Payload, &p); err == nil {
			reasons = append(reasons, p.Reason+":"+p.Text)
		}
	}
	if len(reasons) != 2 {
		t.Fatalf("dequeue markers = %v", reasons)
	}
	close(model.release)
	<-runDone
}
