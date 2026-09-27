package app

import (
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/events"
	controlrpc "agent-vivy/internal/rpc"
	"agent-vivy/internal/runtime"
	"agent-vivy/internal/storage/sqlite"
	"agent-vivy/internal/testsupport"
	"agent-vivy/internal/tools"
)

func TestChildControllerUsesNativeServiceAndCleanOneShotContext(t *testing.T) {
	service, backend, manager, model, sessionID := newNativeChildControllerTest(t, false)
	ctx := context.Background()
	parentID, err := service.Run(ctx, sessionID, "parent-only secret")
	if err != nil {
		t.Fatal(err)
	}
	<-model.parentStarted

	started, err := manager.StartChild(ctx, controlrpc.ChildRequest{
		ParentRunID: string(parentID), Text: "child task only", System: "privileged injected system prompt",
	})
	if err != nil {
		t.Fatalf("start child through native Service: %v", err)
	}
	final, err := manager.WaitChild(ctx, started.ID)
	if err != nil {
		t.Fatalf("wait for native child: %v", err)
	}
	if final.Status != string(domain.RunCompleted) || final.Result != "test response to: child task only" {
		t.Fatalf("native child result = %+v", final)
	}
	if final.ParentRunID != string(parentID) || final.SessionID != string(sessionID) || final.WorkspaceID == "" {
		t.Fatalf("child lineage/workspace = %+v", final)
	}
	listed, err := manager.ListChildren(ctx, string(parentID), false)
	if err != nil || len(listed) != 1 || listed[0].ID != started.ID || listed[0].Result != final.Result {
		t.Fatalf("listed child = %+v err=%v", listed, err)
	}

	inputs := model.snapshot()
	if len(inputs) < 2 {
		t.Fatalf("model calls = %d, want parent and child", len(inputs))
	}
	childInput := inputs[1]
	var childText strings.Builder
	for _, message := range childInput {
		childText.WriteString(message.Content)
		childText.WriteByte('\n')
	}
	if strings.Contains(childText.String(), "parent-only secret") || strings.Contains(childText.String(), "privileged injected system prompt") {
		t.Fatalf("child inherited parent context or caller-supplied system text: %s", childText.String())
	}
	if !strings.Contains(childText.String(), "child task only") {
		t.Fatalf("child task missing from model input: %s", childText.String())
	}

	service.Cancel(parentID)
	if !service.WaitIdle(ctx) {
		t.Fatal("parent run did not stop")
	}
	if _, err := backend.GetRun(ctx, domain.RunID(started.ID)); err != nil {
		t.Fatalf("durable child run missing: %v", err)
	}
}

func TestChildControllerCancellationUsesNativeService(t *testing.T) {
	service, _, manager, model, sessionID := newNativeChildControllerTest(t, true)
	ctx := context.Background()
	parentID, err := service.Run(ctx, sessionID, "parent task")
	if err != nil {
		t.Fatal(err)
	}
	<-model.parentStarted
	started, err := manager.StartChild(ctx, controlrpc.ChildRequest{ParentRunID: string(parentID), Text: "blocked child"})
	if err != nil {
		t.Fatal(err)
	}
	<-model.childStarted

	final, err := manager.CancelChild(ctx, started.ID)
	if err != nil {
		t.Fatalf("cancel child: %v", err)
	}
	if final.Status != string(domain.RunCancelled) {
		t.Fatalf("cancelled child status = %q, want cancelled", final.Status)
	}
	service.Cancel(parentID)
	if !service.WaitIdle(ctx) {
		t.Fatal("parent and child runs did not stop")
	}
}

func TestChildControllerContinuableFollowupKeepsStableSessionAndHistory(t *testing.T) {
	service, _, manager, model, sessionID := newNativeChildControllerTest(t, false)
	ctx := context.Background()
	parentID, err := service.Run(ctx, sessionID, "parent authorizes child")
	if err != nil {
		t.Fatal(err)
	}
	<-model.parentStarted
	started, err := manager.StartChild(ctx, controlrpc.ChildRequest{
		ParentRunID: string(parentID), Text: "first task", Mode: string(domain.ChildModeContinuable), OperationID: "child-create-1",
	})
	if err != nil {
		t.Fatalf("start continuable child: %v", err)
	}
	if started.ChildMode != string(domain.ChildModeContinuable) || started.SessionID == string(sessionID) {
		t.Fatalf("continuable child identity = %+v", started)
	}
	first, err := manager.WaitChild(ctx, started.ID)
	if err != nil || first.Status != string(domain.RunCompleted) {
		t.Fatalf("first activation = %+v err=%v", first, err)
	}

	continued, err := manager.FollowupChild(ctx, controlrpc.ChildFollowupRequest{
		ChildSessionID: started.SessionID, ParentRunID: string(parentID), Text: "follow-up task", OperationID: "child-followup-1",
	})
	if err != nil {
		t.Fatalf("follow up child session: %v", err)
	}
	if continued.ID == first.ID || continued.SessionID != started.SessionID || continued.ParentRunID != string(parentID) {
		t.Fatalf("follow-up activation identity = %+v first=%+v", continued, first)
	}
	last, err := manager.WaitChild(ctx, continued.ID)
	if err != nil || last.Status != string(domain.RunCompleted) {
		t.Fatalf("follow-up activation = %+v err=%v", last, err)
	}
	retry, err := manager.FollowupChild(ctx, controlrpc.ChildFollowupRequest{
		ChildSessionID: started.SessionID, ParentRunID: string(parentID), Text: "follow-up task", OperationID: "child-followup-1",
	})
	if err != nil || retry.ID != continued.ID {
		t.Fatalf("idempotent follow-up = %+v err=%v", retry, err)
	}
	history, err := manager.ChildHistory(ctx, started.SessionID, string(parentID))
	if err != nil || len(history) != 4 || history[0].Content != "first task" || history[2].Content != "follow-up task" {
		t.Fatalf("child history = %+v err=%v", history, err)
	}

	service.Cancel(parentID)
	if !service.WaitIdle(ctx) {
		t.Fatal("parent run did not stop")
	}
}

func TestContinuableChildReplyToolUsesDirectMailboxAndStableCallIdentity(t *testing.T) {
	service, _, manager, model, sessionID := newNativeChildControllerTest(t, true)
	ctx := context.Background()
	parentID, err := service.Run(ctx, sessionID, "parent authorizes child")
	if err != nil {
		t.Fatal(err)
	}
	<-model.parentStarted
	child, err := manager.StartChild(ctx, controlrpc.ChildRequest{
		ParentRunID: string(parentID), Text: "send the parent a result", Mode: string(domain.ChildModeContinuable), OperationID: "reply-tool-child",
	})
	if err != nil {
		t.Fatal(err)
	}
	<-model.childStarted

	tool := tools.NewReplyParent(&replyParentToolRef{manager: manager})
	toolCtx := tools.WithToolCallID(tools.WithSessionID(tools.WithRunID(ctx, domain.RunID(child.ID)), domain.SessionID(child.SessionID)), "call-parent-reply-1")
	args := json.RawMessage(`{"text":"the child result is ready"}`)
	first, err := tool.InvokableRun(toolCtx, args)
	if err != nil {
		t.Fatalf("reply to parent: %v", err)
	}
	retry, err := tool.InvokableRun(toolCtx, args)
	if err != nil || retry != first {
		t.Fatalf("idempotent tool reply = %q, %v; first=%q", retry, err, first)
	}
	inbox, err := service.ListChildMessages(ctx, domain.SessionID(child.SessionID), parentID, 10)
	if err != nil || len(inbox) != 1 || string(inbox[0].Body) != "the child result is ready" || inbox[0].SenderSessionID != domain.SessionID(child.SessionID) {
		t.Fatalf("parent direct inbox = %+v err=%v", inbox, err)
	}
	if _, err := manager.InterruptChild(ctx, child.ID); err != nil {
		t.Fatalf("interrupt child after mailbox assertion: %v", err)
	}
	service.Cancel(parentID)
	if !service.WaitIdle(ctx) {
		t.Fatal("parent and child did not stop")
	}
}

func TestChildInterruptPreservesContinuableSessionAndPendingMail(t *testing.T) {
	service, backend, manager, model, sessionID := newNativeChildControllerTest(t, true)
	ctx := context.Background()
	parentID, err := service.Run(ctx, sessionID, "parent authorizes child")
	if err != nil {
		t.Fatal(err)
	}
	<-model.parentStarted
	started, err := manager.StartChild(ctx, controlrpc.ChildRequest{
		ParentRunID: string(parentID), Text: "blocked child", Mode: string(domain.ChildModeContinuable), OperationID: "interrupt-child-create",
	})
	if err != nil {
		t.Fatal(err)
	}
	<-model.childStarted
	message, inserted, err := manager.SendChildMessage(ctx, controlrpc.ChildMessageRequest{
		ChildSessionID: started.SessionID, ParentRunID: string(parentID), OperationID: "mail-before-interrupt", Text: "please include this next time",
	})
	if err != nil || !inserted || message.Status != string(domain.ChildMessagePending) {
		t.Fatalf("mail admission = %+v inserted=%v err=%v", message, inserted, err)
	}
	if _, err := manager.InterruptChild(ctx, started.ID); err != nil {
		t.Fatalf("interrupt current child activation: %v", err)
	}
	interrupted, err := manager.WaitChild(ctx, started.ID)
	if err != nil || interrupted.Status != string(domain.RunCancelled) {
		t.Fatalf("interrupted activation = %+v err=%v", interrupted, err)
	}
	binding, err := backend.GetChildSessionBinding(ctx, domain.SessionID(started.SessionID))
	if err != nil || binding.State != domain.ChildSessionOpen {
		t.Fatalf("ChildSession after interrupt = %+v err=%v, want open", binding, err)
	}
	pending, err := backend.ListPendingChildMessages(ctx, domain.SessionID(started.SessionID), domain.SessionID(started.SessionID), binding.ConsumedMessageSequence, 10)
	if err != nil || len(pending) != 1 || pending[0].ID != message.ID {
		t.Fatalf("pending mail after interrupt = %+v err=%v, want same admitted message", pending, err)
	}

	continued, err := manager.FollowupChild(ctx, controlrpc.ChildFollowupRequest{
		ChildSessionID: started.SessionID, ParentRunID: string(parentID), Text: "resume with mail", OperationID: "after-interrupt-followup",
	})
	if err != nil {
		t.Fatalf("follow up after interrupt: %v", err)
	}
	result, err := manager.WaitChild(ctx, continued.ID)
	if err != nil || result.Status != string(domain.RunCompleted) {
		t.Fatalf("post-interrupt activation = %+v err=%v", result, err)
	}
	inputs := model.snapshot()
	var foundMail bool
	for _, input := range inputs {
		for _, item := range input {
			if strings.Contains(item.Content, "please include this next time") {
				foundMail = true
			}
		}
	}
	if !foundMail {
		t.Fatalf("admitted mail was not delivered after follow-up: %+v", inputs)
	}
	service.Cancel(parentID)
	if !service.WaitIdle(ctx) {
		t.Fatal("parent run did not stop")
	}
}

type childControllerModel struct {
	inner         *testsupport.EchoModel
	calls         atomic.Int32
	parentStarted chan struct{}
	childStarted  chan struct{}
	releaseParent chan struct{}
	blockChild    bool
	once          sync.Once
	mu            sync.Mutex
	inputs        [][]*domain.Message
}

func newChildControllerModel(blockChild bool) *childControllerModel {
	return &childControllerModel{
		inner: testsupport.NewEchoModel(), parentStarted: make(chan struct{}), childStarted: make(chan struct{}),
		releaseParent: make(chan struct{}), blockChild: blockChild,
	}
}

func (m *childControllerModel) Stream(ctx context.Context, input []*domain.Message) (domain.Stream[*domain.Message], error) {
	call := m.calls.Add(1)
	copyInput := make([]*domain.Message, len(input))
	for i, message := range input {
		if message != nil {
			copy := *message
			copyInput[i] = &copy
		}
	}
	m.mu.Lock()
	m.inputs = append(m.inputs, copyInput)
	m.mu.Unlock()
	switch call {
	case 1:
		m.once.Do(func() { close(m.parentStarted) })
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-m.releaseParent:
		}
	case 2:
		close(m.childStarted)
		if m.blockChild {
			<-ctx.Done()
			return nil, ctx.Err()
		}
	}
	return m.inner.Stream(ctx, input)
}

func (m *childControllerModel) snapshot() [][]*domain.Message {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([][]*domain.Message, len(m.inputs))
	for i, input := range m.inputs {
		out[i] = append([]*domain.Message(nil), input...)
	}
	return out
}

func newNativeChildControllerTest(t *testing.T, blockChild bool) (*runtime.Service, *sqlite.Backend, *workerManager, *childControllerModel, domain.SessionID) {
	t.Helper()
	ctx := context.Background()
	backend, err := sqlite.Open(ctx, filepath.Join(t.TempDir(), "children.db"))
	if err != nil {
		t.Fatal(err)
	}
	if err := backend.CreateSession(ctx, domain.Session{ID: "parent-session", Title: "parent", CreatedAt: time.Now().UnixMilli()}); err != nil {
		t.Fatal(err)
	}
	replyOps := &replyParentToolRef{}
	registry := tools.NewRegistry(tools.NewEchoInfo(), tools.NewReplyParent(replyOps))
	selected, err := registry.Resolve([]string{tools.EchoInfoName, tools.ReplyParentName})
	if err != nil {
		t.Fatal(err)
	}
	policy, err := runtime.NewPolicyEngine(nil)
	if err != nil {
		t.Fatal(err)
	}
	hooks := runtime.NewToolHookChain(time.Second)
	model := newChildControllerModel(blockChild)
	engine, err := runtime.NewEngine(ctx, runtime.WrapModel(model), selected, runtime.EngineConfig{
		StreamBuffer: 8, MaxEventPayloadBytes: 64 << 10, Policy: policy, ToolHooks: hooks,
	})
	if err != nil {
		t.Fatal(err)
	}
	workspaces, err := runtime.NewWorkspaceManager(filepath.Join(t.TempDir(), "workspaces"))
	if err != nil {
		t.Fatal(err)
	}
	service := runtime.NewService(engine, "test", "test-model", runtime.ServiceDeps{
		Journal: backend, Runs: backend, Messages: backend, Approvals: backend, Questions: backend,
		Sessions: backend, Workspaces: workspaces, Sink: events.NewBus(8),
		PolicyDefaultProfile: domain.PolicyProfileDefault, Budget: runtime.DefaultBudgetPolicy(),
	})
	manager := newWorkerManager(service, backend)
	replyOps.arm(manager)
	t.Cleanup(func() {
		service.CancelAll()
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		if !service.WaitIdle(cleanupCtx) {
			t.Errorf("service did not drain: %v", cleanupCtx.Err())
		}
		_ = backend.Close()
	})
	return service, backend, manager, model, "parent-session"
}
