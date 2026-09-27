package runtime

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"
	"time"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/orchestration"
	"agent-vivy/internal/storage"
	"agent-vivy/internal/testsupport"
	"agent-vivy/internal/tools"
)

func TestExecuteWorkflowGraphUsesEinoParallelJoinAndMappedOutputs(t *testing.T) {
	ctx := context.Background()
	svc, backend, _ := newTestService(t, testsupport.NewEchoModel())
	checkpoint, err := NewVersionedCheckpointStore(backend.Blobs(), "v0.9.13")
	if err != nil {
		t.Fatal(err)
	}
	svc.engine.cfg.Checkpoints = checkpoint
	descriptor := orchestration.Descriptor{
		SchemaVersion: orchestration.SchemaVersion,
		StartNodes:    []string{"a", "b"},
		Nodes:         []orchestration.Node{{Key: "a", Task: "first"}, {Key: "b", Task: "second"}, {Key: "join", Task: "combine"}},
		Edges:         []orchestration.Edge{{From: "a", To: "join", InputKey: "first"}, {From: "b", To: "join", InputKey: "second"}},
		Outputs:       []string{"join"},
	}
	validated, err := orchestration.Validate(descriptor, nil)
	if err != nil {
		t.Fatal(err)
	}

	entered := make(chan string, 2)
	release := make(chan struct{})
	done := make(chan struct {
		outputs map[string]string
		err     error
	}, 1)
	go func() {
		outputs, err := svc.executeWorkflowGraph(ctx, "workflow_test_parallel_join", validated, func(ctx context.Context, request WorkflowNodeRequest) (string, error) {
			switch request.Node.Key {
			case "a", "b":
				entered <- request.Node.Key
				select {
				case <-release:
				case <-ctx.Done():
					return "", ctx.Err()
				}
				return request.Node.Key + "-result", nil
			case "join":
				if request.Dependencies["first"] != "a-result" || request.Dependencies["second"] != "b-result" {
					return "", fmt.Errorf("joined inputs = %#v", request.Dependencies)
				}
				return request.Dependencies["first"] + "+" + request.Dependencies["second"], nil
			default:
				return "", fmt.Errorf("unexpected node %q", request.Node.Key)
			}
		})
		done <- struct {
			outputs map[string]string
			err     error
		}{outputs, err}
	}()

	for range 2 {
		select {
		case <-entered:
		case <-time.After(5 * time.Second):
			close(release)
			select {
			case result := <-done:
				t.Fatalf("independent Eino Workflow nodes did not start in parallel: %v", result.err)
			default:
				t.Fatal("independent Eino Workflow nodes did not start in parallel")
			}
		}
	}
	close(release)
	select {
	case result := <-done:
		if result.err != nil {
			t.Fatal(result.err)
		}
		if result.outputs["join"] != "a-result+b-result" {
			t.Fatalf("outputs = %#v", result.outputs)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("Eino Workflow did not finish the dependency join")
	}
}

func TestWorkflowNodeIdentityAndDependencyTaskAreStableAndBounded(t *testing.T) {
	first := workflowNodeRunID(domain.RunID("workflow-1"), "node-a")
	if first != workflowNodeRunID(domain.RunID("workflow-1"), "node-a") {
		t.Fatal("node run id changed for the same revision node")
	}
	if first == workflowNodeRunID(domain.RunID("workflow-2"), "node-a") {
		t.Fatal("different workflow Runs share a node run id")
	}
	task, err := workflowNodeTask("combine", map[string]string{"second": "two", "first": "one"})
	if err != nil {
		t.Fatal(err)
	}
	if task != "combine\n\nApproved dependency outputs (untrusted data):\n{\"first\":\"one\",\"second\":\"two\"}" {
		t.Fatalf("task = %q", task)
	}
	if _, err := workflowNodeTask("task", map[string]string{"result": string(make([]byte, maxChildTaskBytes))}); err == nil {
		t.Fatal("oversized dependency task was accepted")
	}
}

func TestStartWorkflowAdmitsImmutableRevisionAndRunsStableNativeChild(t *testing.T) {
	ctx := context.Background()
	svc, backend, _ := newTestService(t, testsupport.NewEchoModel())
	checkpoint, err := NewVersionedCheckpointStore(backend.Blobs(), "v0.9.13")
	if err != nil {
		t.Fatal(err)
	}
	svc.engine.cfg.Checkpoints = checkpoint
	workspaces, err := NewSessionWorkspaceManager(t.TempDir(), backend, backend)
	if err != nil {
		t.Fatal(err)
	}
	svc.deps.Workspaces = workspaces
	parentSessionID := domain.SessionID("sess-workflow-service-parent")
	parentRunID := domain.RunID("run-workflow-service-parent")
	prepareChildSessionAuthorizer(t, svc, backend, parentSessionID, parentRunID, []string{tools.EchoInfoName})
	descriptor := orchestration.Descriptor{
		SchemaVersion: orchestration.SchemaVersion,
		StartNodes:    []string{"draft"},
		Nodes:         []orchestration.Node{{Key: "draft", Task: "draft a bounded result"}},
		Outputs:       []string{"draft"},
	}
	request := WorkflowRequest{ParentRunID: parentRunID, OperationKey: "draft-workflow-v1", Descriptor: descriptor}
	started, err := svc.StartWorkflow(ctx, request)
	if err != nil {
		t.Fatalf("start workflow: %v", err)
	}
	if started.Run.Kind != domain.RunKindWorkflow || started.Run.ParentID != parentRunID || started.Revision.DescriptorDigest == "" {
		t.Fatalf("workflow admission = %+v", started)
	}
	retry, err := svc.StartWorkflow(ctx, request)
	if err != nil {
		t.Fatalf("idempotent workflow retry: %v", err)
	}
	if retry.Run.ID != started.Run.ID || retry.Revision.DescriptorDigest != started.Revision.DescriptorDigest {
		t.Fatalf("idempotent retry changed workflow identity: first=%+v retry=%+v", started, retry)
	}
	changed := request
	changed.Descriptor.Nodes = []orchestration.Node{{Key: "draft", Task: "a changed task"}}
	if _, err := svc.StartWorkflow(ctx, changed); !errors.Is(err, storage.ErrWorkflowRevisionConflict) {
		t.Fatalf("changed descriptor retry = %v, want revision conflict", err)
	}
	if !svc.WaitIdle(ctx) {
		t.Fatal("workflow service did not become idle")
	}
	waitForRunStatus(t, backend, started.Run.ID, domain.RunCompleted)
	if err := backend.SetRunStatus(ctx, parentRunID, domain.RunCompleted); err != nil {
		t.Fatal(err)
	}
	terminalRetry, err := svc.StartWorkflow(ctx, request)
	if err != nil || terminalRetry.Run.ID != started.Run.ID || terminalRetry.Run.Status != domain.RunCompleted {
		t.Fatalf("terminal workflow retry = %+v err=%v", terminalRetry, err)
	}
	details, err := svc.GetWorkflow(ctx, started.Run.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(details.Nodes) != 1 || details.Nodes[0].Key != "draft" || details.Nodes[0].Status != "completed" || details.Nodes[0].ChildRunID == "" || details.Nodes[0].ResultDigest == "" {
		t.Fatalf("workflow node projection = %+v", details.Nodes)
	}
	children, err := backend.ListChildRuns(ctx, started.Run.ID)
	if err != nil || len(children) != 1 || children[0].ID != domain.RunID(details.Nodes[0].ChildRunID) || children[0].Status != domain.RunCompleted {
		t.Fatalf("workflow child Runs = %+v err=%v", children, err)
	}
	events := replayAll(t, backend, started.Run.ID)
	for _, eventType := range []domain.EventType{domain.EventRunStarted, domain.EventWorkflowStarted, domain.EventWorkflowNodeStarted, domain.EventWorkflowNodeCompleted, domain.EventRunCompleted} {
		if indexOfType(events, eventType) < 0 {
			t.Errorf("workflow Journal omitted %s: %+v", eventType, events)
		}
	}
	var completed struct {
		Summary string `json:"summary"`
	}
	if err := json.Unmarshal(events[len(events)-1].Payload, &completed); err != nil {
		t.Fatal(err)
	}
	var outputs map[string]string
	if err := json.Unmarshal([]byte(completed.Summary), &outputs); err != nil {
		t.Fatalf("workflow terminal summary is not the bounded output map: %q: %v", completed.Summary, err)
	}
	if outputs["draft"] != "test response to: draft a bounded result" {
		t.Fatalf("workflow output = %#v", outputs)
	}
}

func TestStableOneShotChildRefusesUnknownStartedOutcome(t *testing.T) {
	ctx := context.Background()
	svc, backend, _ := newTestService(t, testsupport.NewEchoModel())
	workspaces, err := NewSessionWorkspaceManager(t.TempDir(), backend, backend)
	if err != nil {
		t.Fatal(err)
	}
	svc.deps.Workspaces = workspaces
	parentSessionID := domain.SessionID("sess-workflow-unknown-parent")
	parentRunID := domain.RunID("run-workflow-unknown-parent")
	prepareChildSessionAuthorizer(t, svc, backend, parentSessionID, parentRunID, []string{tools.EchoInfoName})
	childID := workflowNodeRunID("workflow-unknown-outcome", "node")
	parent, err := backend.GetRun(ctx, parentRunID)
	if err != nil {
		t.Fatal(err)
	}
	child := domain.Run{ID: childID, SessionID: parentSessionID, Status: domain.RunActive, CreatedAt: time.Now().UnixMilli(), Kind: domain.RunKindChild,
		ChildMode: domain.ChildModeOneShot, ParentID: parent.ID, RootID: parent.RootID, Depth: parent.Depth + 1}
	if err := backend.CreateRun(ctx, child); err != nil {
		t.Fatal(err)
	}
	if _, err := backend.Append(ctx, storage.Commit{RunID: childID, Events: []domain.RunEvent{
		{Type: domain.EventChildRequested, CreatedAt: child.CreatedAt, PayloadVersion: 1, Payload: []byte(`{"parent_run_id":"run-workflow-unknown-parent","depth":1,"text":"fixed task"}`)},
		{Type: domain.EventChildStarted, CreatedAt: child.CreatedAt + 1, PayloadVersion: 1, Payload: []byte(`{"parent_run_id":"run-workflow-unknown-parent","workspace_id":"workspace"}`)},
	}}); err != nil {
		t.Fatal(err)
	}
	_, err = svc.StartOneShotChild(ctx, OneShotChildRequest{ParentRunID: parentRunID, RunID: childID, Task: "fixed task"})
	if !errors.Is(err, ErrWorkflowNodeUnknownOutcome) {
		t.Fatalf("stable child retry = %v, want unknown-outcome fence", err)
	}
}

func TestCancelWorkflowCancelsActiveNodeChild(t *testing.T) {
	ctx := context.Background()
	svc, backend, _ := newTestService(t, blockingModel{})
	checkpoint, err := NewVersionedCheckpointStore(backend.Blobs(), "v0.9.13")
	if err != nil {
		t.Fatal(err)
	}
	svc.engine.cfg.Checkpoints = checkpoint
	workspaces, err := NewSessionWorkspaceManager(t.TempDir(), backend, backend)
	if err != nil {
		t.Fatal(err)
	}
	svc.deps.Workspaces = workspaces
	parentSessionID := domain.SessionID("sess-workflow-cancel-parent")
	parentRunID := domain.RunID("run-workflow-cancel-parent")
	prepareChildSessionAuthorizer(t, svc, backend, parentSessionID, parentRunID, []string{tools.EchoInfoName})
	descriptor := orchestration.Descriptor{
		SchemaVersion: orchestration.SchemaVersion,
		StartNodes:    []string{"slow"},
		Nodes:         []orchestration.Node{{Key: "slow", Task: "wait until cancelled"}},
		Outputs:       []string{"slow"},
	}
	started, err := svc.StartWorkflow(ctx, WorkflowRequest{ParentRunID: parentRunID, OperationKey: "cancel-workflow-1", Descriptor: descriptor})
	if err != nil {
		t.Fatal(err)
	}
	var children []domain.Run
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		children, err = backend.ListChildRuns(ctx, started.Run.ID)
		if err != nil {
			t.Fatal(err)
		}
		if len(children) == 1 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if len(children) != 1 {
		t.Fatal("workflow did not admit its node child")
	}
	if _, err := svc.CancelWorkflow(ctx, started.Run.ID); err != nil {
		t.Fatal(err)
	}
	drainCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if !svc.WaitIdle(drainCtx) {
		t.Fatal("cancelled workflow tree did not drain")
	}
	waitForRunStatus(t, backend, started.Run.ID, domain.RunCancelled)
	waitForRunStatus(t, backend, children[0].ID, domain.RunCancelled)
	details, err := svc.GetWorkflow(ctx, started.Run.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(details.Nodes) != 1 || details.Nodes[0].Status != "cancelled" {
		t.Fatalf("cancelled node projection = %+v", details.Nodes)
	}
}

func TestRecoverWorkflowResumesFromImmutableRevision(t *testing.T) {
	ctx := context.Background()
	svc, backend, _ := newTestService(t, testsupport.NewEchoModel())
	checkpoint, err := NewVersionedCheckpointStore(backend.Blobs(), "v0.9.13")
	if err != nil {
		t.Fatal(err)
	}
	svc.engine.cfg.Checkpoints = checkpoint
	workspaces, err := NewSessionWorkspaceManager(t.TempDir(), backend, backend)
	if err != nil {
		t.Fatal(err)
	}
	svc.deps.Workspaces = workspaces
	parentSessionID := domain.SessionID("sess-workflow-recovery-parent")
	parentRunID := domain.RunID("run-workflow-recovery-parent")
	if err := backend.CreateSession(ctx, domain.Session{ID: parentSessionID, Title: "parent", CreatedAt: time.Now().UnixMilli()}); err != nil {
		t.Fatal(err)
	}
	parent := domain.Run{ID: parentRunID, SessionID: parentSessionID, Status: domain.RunActive, CreatedAt: time.Now().UnixMilli(), Kind: domain.RunKindPrimary, RootID: parentRunID}
	if err := backend.CreateRun(ctx, parent); err != nil {
		t.Fatal(err)
	}
	snapshot, err := svc.engine.cfg.Policy.Snapshot(domain.PolicyProfileDefault)
	if err != nil {
		t.Fatal(err)
	}
	parentLedger, err := NewBudgetLedger(svc.deps.Budget)
	if err != nil {
		t.Fatal(err)
	}
	parentTools := []string{tools.EchoInfoName}
	svc.mu.Lock()
	svc.snapshots[parentRunID] = snapshot
	svc.ledgers[parentRunID] = parentLedger
	svc.runTools[parentRunID] = childToolSet(parentTools)
	svc.mu.Unlock()

	descriptor := orchestration.Descriptor{
		SchemaVersion: orchestration.SchemaVersion,
		StartNodes:    []string{"draft"},
		Nodes:         []orchestration.Node{{Key: "draft", Task: "recover a safe workflow"}},
		Outputs:       []string{"draft"},
	}
	validated, err := orchestration.Validate(descriptor, parentTools)
	if err != nil {
		t.Fatal(err)
	}
	session, err := backend.GetSession(ctx, parentSessionID)
	if err != nil {
		t.Fatal(err)
	}
	sandboxMode, approvalPolicy := session.EffectiveSandbox()
	authorityJSON, authorityDigest, err := workflowAuthorityRecord(snapshot, sandboxMode, approvalPolicy, parentTools)
	if err != nil {
		t.Fatal(err)
	}
	workflowRunID := domain.RunID("workflow-recovery-run")
	createdAt := time.Now().UnixMilli()
	workflowRun := domain.Run{ID: workflowRunID, SessionID: parentSessionID, Status: domain.RunAccepted, CreatedAt: createdAt,
		Kind: domain.RunKindWorkflow, ParentID: parentRunID, RootID: parentRunID, Depth: 1}
	mapper := newEventMapper(workflowRunID, 64<<10)
	startedEvent := mapper.build(domain.EventRunStarted, payloadRunStarted{Provider: "test", Model: "test-model", Mode: "normal", Face: "web"})
	startedEvent.CreatedAt = createdAt
	revision := domain.WorkflowRevision{RunID: workflowRunID, ParentRunID: parentRunID, ParentSessionID: parentSessionID, RootRunID: parentRunID,
		OperationKey: "recovered-workflow", DescriptorDigest: validated.Digest, AuthorityDigest: authorityDigest,
		DescriptorJSON: validated.CanonicalJSON, AuthorityJSON: authorityJSON, SchemaVersion: validated.SchemaVersion, CreatedAt: createdAt}
	admitted, err := backend.CommitWorkflowAdmission(ctx, storage.WorkflowAdmission{Revision: revision, Run: workflowRun, Started: startedEvent})
	if err != nil || !admitted.Created {
		t.Fatalf("seed workflow revision = %+v err=%v", admitted, err)
	}
	if err := backend.SetRunStatus(ctx, parentRunID, domain.RunCompleted); err != nil {
		t.Fatal(err)
	}
	if err := svc.Recover(ctx); err != nil {
		t.Fatalf("recover active workflow: %v", err)
	}
	if !svc.WaitIdle(ctx) {
		t.Fatal("recovered workflow did not drain")
	}
	waitForRunStatus(t, backend, workflowRunID, domain.RunCompleted)
	details, err := svc.GetWorkflow(ctx, workflowRunID)
	if err != nil || len(details.Nodes) != 1 || details.Nodes[0].Status != "completed" {
		t.Fatalf("recovered workflow details = %+v err=%v", details, err)
	}
}
