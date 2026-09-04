package app

import (
	"context"
	"encoding/json"
	"errors"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/cloudwego/eino/schema"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/events"
	controlrpc "agent-vivy/internal/rpc"
	"agent-vivy/internal/runtime"
	"agent-vivy/internal/storage/sqlite"
	"agent-vivy/internal/testsupport"
	"agent-vivy/internal/tools"
	"agent-vivy/internal/worker"
)

func TestLegacyWorkerModelBrokerEmitsBoundedV2AssistantEvents(t *testing.T) {
	manager, backend, _, child := newChildBrokerTest(t)
	content := strings.Repeat("worker Unicode 正文 ", 12000)
	inner, err := runtime.NewWorkerModelBroker(runtime.NewScriptedModel(schema.AssistantMessage(content, nil)), nil)
	if err != nil {
		t.Fatal(err)
	}
	broker := &legacyModelBroker{inner: inner, manager: manager, childID: child.ID, ledger: manager.serviceLedger(child.ID)}
	if _, err := broker.Complete(context.Background(), worker.ModelRequest{
		RunID: string(child.ID), ParentRunID: string(child.ParentID),
		Messages: []worker.ChatMessage{{Role: "user", Content: "answer"}},
	}); err != nil {
		t.Fatal(err)
	}
	it, err := backend.Replay(context.Background(), child.ID, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer it.Close()
	var body strings.Builder
	completed := 0
	for it.Next() {
		event := it.Value().Event
		if len(event.Payload) > manager.service.MaxEventPayloadBytes() {
			t.Fatalf("%s payload = %d bytes, limit %d", event.Type, len(event.Payload), manager.service.MaxEventPayloadBytes())
		}
		switch event.Type {
		case domain.EventModelDelta:
			var payload struct {
				Delta string `json:"delta"`
			}
			if err := json.Unmarshal(event.Payload, &payload); err != nil {
				t.Fatal(err)
			}
			body.WriteString(payload.Delta)
		case domain.EventModelCompleted:
			completed++
			if event.PayloadVersion != 2 || strings.Contains(string(event.Payload), `"content"`) {
				t.Fatalf("completion = version %d payload %s", event.PayloadVersion, event.Payload)
			}
		}
	}
	if err := it.Err(); err != nil {
		t.Fatal(err)
	}
	if body.String() != content || completed != 1 {
		t.Fatalf("worker projection bytes/completions = %d/%d, want %d/1", body.Len(), completed, len(content))
	}
}

func TestLegacyWorkerModelBrokerPersistsAttributedCachedUsage(t *testing.T) {
	manager, backend, _, child := newChildBrokerTest(t)
	message := schema.AssistantMessage("done", nil)
	message.ResponseMeta = &schema.ResponseMeta{Usage: &schema.TokenUsage{
		PromptTokens: 10, CompletionTokens: 5, TotalTokens: 15,
		PromptTokenDetails:      schema.PromptTokenDetails{CachedTokens: 4},
		CompletionTokensDetails: schema.CompletionTokensDetails{ReasoningTokens: 2},
	}}
	inner, err := runtime.NewWorkerModelBroker(runtime.NewScriptedModel(message), nil)
	if err != nil {
		t.Fatal(err)
	}
	broker := &legacyModelBroker{inner: inner, manager: manager, childID: child.ID, ledger: manager.serviceLedger(child.ID)}
	response, err := broker.Complete(context.Background(), worker.ModelRequest{
		RunID: string(child.ID), ParentRunID: string(child.ParentID), Messages: []worker.ChatMessage{{Role: "user", Content: "answer"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if response.Usage == nil || response.Usage.CachedTokens != 4 {
		t.Fatalf("worker response usage = %+v", response.Usage)
	}
	rows, err := backend.ListModelUsage(context.Background(), 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0].Provider != "test" || rows[0].Model != "test-model" || rows[0].Source != "child" || rows[0].CachedTokens != 4 || rows[0].ReasoningTokens != 2 {
		t.Fatalf("projected child usage = %+v", rows)
	}
}

func TestChildModelDeltasDoNotExhaustSemanticEventBudget(t *testing.T) {
	manager, backend, _, child := newChildBrokerTest(t)
	ledger, err := runtime.NewBudgetLedger(runtime.BudgetPolicy{
		MaxEvents: 1, MaxModelCalls: 1, MaxToolCalls: 1, MaxRetries: 1,
	})
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 600; i++ {
		if err := manager.recordChildEventVersion(context.Background(), child.ID, ledger, domain.EventModelDelta, 1, map[string]any{"delta": "字"}); err != nil {
			t.Fatalf("delta %d consumed semantic event budget: %v", i, err)
		}
	}
	if err := manager.recordChildEventVersion(context.Background(), child.ID, ledger, domain.EventModelCompleted, 2, map[string]any{
		"content_sha256": strings.Repeat("0", 64), "byte_len": 1800,
	}); err != nil {
		t.Fatalf("completion should consume the one semantic event slot: %v", err)
	}
	if err := manager.recordChildEvent(context.Background(), child.ID, ledger, domain.EventModelUsage, map[string]any{}); !errors.Is(err, runtime.ErrBudgetExceeded) {
		t.Fatalf("next semantic event error = %v, want budget exceeded", err)
	}
	events := replayWorkerEvents(t, backend, child.ID)
	if got := len(events); got != 601 {
		t.Fatalf("persisted child events = %d, want 601", got)
	}
}

func replayWorkerEvents(t *testing.T, backend *sqlite.Backend, runID domain.RunID) []domain.RunEvent {
	t.Helper()
	it, err := backend.Replay(context.Background(), runID, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer it.Close()
	var out []domain.RunEvent
	for it.Next() {
		out = append(out, it.Value().Event)
	}
	if err := it.Err(); err != nil {
		t.Fatal(err)
	}
	return out
}

func newChildBrokerTest(t *testing.T) (*workerManager, *sqlite.Backend, domain.Run, domain.Run) {
	t.Helper()
	ctx := context.Background()
	backend, err := sqlite.Open(ctx, filepath.Join(t.TempDir(), "child.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = backend.Close() })
	if err := backend.CreateSession(ctx, domain.Session{ID: "sess-child", Title: "child", CreatedAt: time.Now().UnixMilli()}); err != nil {
		t.Fatal(err)
	}
	policy, err := runtime.NewPolicyEngine(nil)
	if err != nil {
		t.Fatal(err)
	}
	workspace, err := runtime.NewWorkspaceManager(filepath.Join(t.TempDir(), "workspaces"))
	if err != nil {
		t.Fatal(err)
	}
	model := runtime.WrapModel(testsupport.NewEchoModel())
	engine, err := runtime.NewEngine(ctx, model, nil, runtime.EngineConfig{StreamBuffer: 8, MaxEventPayloadBytes: 64 << 10})
	if err != nil {
		t.Fatal(err)
	}
	bus := events.NewBus(8)
	service := runtime.NewService(engine, "test", "test-model", runtime.ServiceDeps{
		Journal: backend, Runs: backend, Messages: backend, Approvals: backend, Questions: backend,
		Workspaces: workspace, Sessions: backend, Sink: bus,
	})
	manager := newWorkerManager(service, backend, backend, policy, runtime.NewToolHookChain(time.Second), []tools.Tool{tools.NewWriteNote(backend)}, 4096, time.Minute, model, worker.WorkerLog{})
	service.SetChildApprovalRouter(manager)
	service.SetChildRunCanceller(manager)
	snapshot, err := policy.Snapshot(domain.PolicyProfileDefault)
	if err != nil {
		t.Fatal(err)
	}
	rootLedger, err := runtime.NewBudgetLedger(runtime.DefaultBudgetPolicy())
	if err != nil {
		t.Fatal(err)
	}
	root := domain.Run{ID: "run-root", SessionID: "sess-child", Status: domain.RunActive, CreatedAt: time.Now().UnixMilli(), Kind: domain.RunKindPrimary, RootID: "run-root"}
	child := domain.Run{ID: "child-test", SessionID: "sess-child", Status: domain.RunActive, CreatedAt: time.Now().UnixMilli(), Kind: domain.RunKindChild, ParentID: root.ID, RootID: root.ID, Depth: 1}
	if err := backend.CreateRun(ctx, root); err != nil {
		t.Fatal(err)
	}
	if err := backend.CreateRun(ctx, child); err != nil {
		t.Fatal(err)
	}
	if err := service.RegisterWorkerAuthority(root.ID, snapshot, rootLedger); err != nil {
		t.Fatal(err)
	}
	childLedger, err := rootLedger.Child(runtime.DefaultBudgetPolicy())
	if err != nil {
		t.Fatal(err)
	}
	if err := service.RegisterWorkerAuthority(child.ID, snapshot, childLedger); err != nil {
		t.Fatal(err)
	}
	return manager, backend, root, child
}

func TestDeleteSessionCancelsLiveChildWorker(t *testing.T) {
	manager, _, _, child := newChildBrokerTest(t)
	cancelled := make(chan struct{})
	var once sync.Once
	manager.mu.Lock()
	manager.children[child.ID] = &childHandle{
		run: child, done: make(chan struct{}),
		cancel: func() { once.Do(func() { close(cancelled) }) },
	}
	manager.mu.Unlock()
	if err := manager.service.DeleteSession(context.Background(), child.SessionID); err != nil {
		t.Fatal(err)
	}
	select {
	case <-cancelled:
	case <-time.After(time.Second):
		t.Fatal("session deletion did not cancel live child worker")
	}
}

func TestChildToolBrokerApprovalResumesAndExecutes(t *testing.T) {
	manager, backend, _, child := newChildBrokerTest(t)
	ctx := context.Background()
	broker := &childToolBroker{manager: manager, childID: child.ID, profile: domain.PolicyProfileDefault, ledger: manager.serviceLedger(child.ID)}
	call := worker.ToolCall{RunID: child.ID, ParentRunID: child.ParentID, PolicyProfile: domain.PolicyProfileDefault, ToolName: tools.WriteNoteName, CallID: "call-1", Args: map[string]any{"content": "approval path"}}
	first, err := broker.Execute(ctx, call)
	if err != nil || first.Status != "approval_required" || first.ApprovalID == "" {
		t.Fatalf("first tool result = %+v, err = %v", first, err)
	}
	approval, err := backend.GetApproval(ctx, first.ApprovalID)
	if err != nil || approval.Kind != domain.ApprovalKindChild || approval.RunID != child.ID {
		t.Fatalf("approval = %+v, err = %v", approval, err)
	}
	waitResult := make(chan worker.ApprovalWaitResult, 1)
	go func() {
		result, waitErr := manager.Wait(ctx, worker.ApprovalWaitRequest{ApprovalID: approval.ID, RunID: string(child.ID), ToolCallID: call.CallID})
		if waitErr != nil {
			t.Errorf("wait approval: %v", waitErr)
			return
		}
		waitResult <- result
	}()
	if err := manager.service.DecideApproval(ctx, approval.ID, domain.ApprovalApproved); err != nil {
		t.Fatal(err)
	}
	select {
	case result := <-waitResult:
		if result.Decision != domain.ApprovalApproved {
			t.Fatalf("approval decision = %+v", result)
		}
	case <-time.After(time.Second):
		t.Fatal("approval waiter did not resume")
	}
	call.ApprovalID = approval.ID
	second, err := broker.Execute(ctx, call)
	if err != nil || second.Status != "completed" {
		t.Fatalf("approved tool result = %+v, err = %v", second, err)
	}
	notes, err := backend.ListNotes(ctx)
	if err != nil || len(notes) != 1 || notes[0].Content != "approval path" {
		t.Fatalf("notes = %+v, err = %v", notes, err)
	}
}

func TestChildStartRejectsDepthAndConcurrencyBeforeSpawning(t *testing.T) {
	manager, _, root, _ := newChildBrokerTest(t)
	ctx := context.Background()
	manager.parentCounts[root.ID] = maxChildrenPerParent
	if _, err := manager.StartChild(ctx, controlrpc.ChildRequest{ParentRunID: string(root.ID), Text: "blocked"}); err == nil {
		t.Fatal("parent concurrency limit must reject the child")
	}
	manager.parentCounts[root.ID] = 0
	depthRoot := root
	depthRoot.ID = "run-depth"
	depthRoot.Depth = maxChildDepth
	depthRoot.RootID = depthRoot.ID
	if err := manager.runs.CreateRun(ctx, depthRoot); err != nil {
		t.Fatal(err)
	}
	if err := manager.service.RegisterWorkerAuthority(depthRoot.ID, rootSnapshot(manager, root.ID), rootLedger(manager, root.ID)); err != nil {
		t.Fatal(err)
	}
	if _, err := manager.StartChild(ctx, controlrpc.ChildRequest{ParentRunID: string(depthRoot.ID), Text: "blocked"}); err == nil {
		t.Fatal("depth limit must reject the child")
	}
}

func (m *workerManager) serviceLedger(id domain.RunID) *runtime.BudgetLedger {
	return m.serviceLedgerForTest(id)
}

func (m *workerManager) serviceLedgerForTest(id domain.RunID) *runtime.BudgetLedger {
	// The test is in package app and can use the runtime authority seam via a
	// fresh child scope; the actual child manager always receives this ledger
	// from WorkerChildAuthority.
	_, ledger, _, err := m.service.WorkerParentAuthority(context.Background(), id)
	if err != nil {
		return nil
	}
	return ledger
}

func rootSnapshot(m *workerManager, id domain.RunID) domain.PolicySnapshot {
	snapshot, _, _, err := m.service.WorkerParentAuthority(context.Background(), id)
	if err != nil {
		return domain.PolicySnapshot{}
	}
	return snapshot
}

func rootLedger(m *workerManager, id domain.RunID) *runtime.BudgetLedger {
	_, ledger, _, err := m.service.WorkerParentAuthority(context.Background(), id)
	if err != nil {
		return nil
	}
	return ledger
}
