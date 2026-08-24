package app

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/events"
	"agent-vivy/internal/provider"
	controlrpc "agent-vivy/internal/rpc"
	"agent-vivy/internal/runtime"
	"agent-vivy/internal/storage/sqlite"
	"agent-vivy/internal/tools"
	"agent-vivy/internal/worker"
)

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
	model := runtime.WrapModel(provider.NewMock())
	engine, err := runtime.NewEngine(ctx, model, nil, runtime.EngineConfig{StreamBuffer: 8, MaxEventPayloadBytes: 64 << 10})
	if err != nil {
		t.Fatal(err)
	}
	bus := events.NewBus(8)
	service := runtime.NewService(engine, "mock", "mock", runtime.ServiceDeps{
		Journal: backend, Runs: backend, Messages: backend, Approvals: backend, Questions: backend,
		Workspaces: workspace, Sink: bus,
	})
	manager := newWorkerManager(service, backend, backend, policy, runtime.NewToolHookChain(time.Second), []tools.Tool{tools.NewWriteNote(backend)}, 4096, time.Minute, model)
	service.SetChildApprovalRouter(manager)
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
