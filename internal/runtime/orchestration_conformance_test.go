package runtime

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/cloudwego/eino/compose"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/maskcontract"
	"agent-vivy/internal/storage"
	"agent-vivy/internal/storage/sqlite"
	"agent-vivy/internal/testsupport"
)

func TestOrchestrationNative(t *testing.T) {
	ctx := context.Background()
	svc, backend, _ := newTestService(t, testsupport.NewEchoModel())
	sessionID := domain.SessionID("session-native-orchestration")
	if err := backend.CreateSession(ctx, domain.Session{ID: sessionID, Title: "native orchestration", CreatedAt: time.Now().UnixMilli()}); err != nil {
		t.Fatalf("create session: %v", err)
	}
	svc.deps.Admission = backend
	svc.deps.GenerationID = "test-generation"
	runID := domain.RunID(newRunID())
	checkpointStore, err := NewVersionedCheckpointStore(backend.Blobs(), "v0.9.13")
	if err != nil {
		t.Fatalf("create checkpoint store: %v", err)
	}
	svc.engine.cfg.Checkpoints = checkpointStore

	// This immutable prompt and graph Run are the shared identity boundary for
	// every workflow node and the Eino checkpoint.
	prompt, err := buildPromptSnapshot(PromptInput{
		RunID: runID, GenerationID: svc.deps.GenerationID,
		Capture: maskcontract.Capture{Selection: maskcontract.Selection{SessionID: sessionID}},
	})
	if err != nil {
		t.Fatalf("build prompt snapshot: %v", err)
	}
	descriptorBytes, revision, err := nativeOrchestrationDescriptor("alpha", "beta")
	if err != nil {
		t.Fatalf("build workflow descriptor: %v", err)
	}
	now := time.Now().UnixMilli()
	mapper := newEventMapper(runID, 64<<10)
	started := mapper.build(domain.EventRunStarted, payloadRunStarted{
		Provider: "test", Model: "test-model", Mode: string(domain.RunModeNormal), Face: string(domain.FaceWeb),
		PolicyProfile: string(domain.PolicyProfileDefault), PolicyHash: "test-policy",
		PromptSchema: prompt.SchemaVersion, PromptDigest: prompt.PayloadSHA256,
	})
	_, err = backend.CommitRunAdmission(ctx, storage.RunAdmission{
		Message: domain.Message{
			ID: newMessageID(), SessionID: sessionID, RunID: runID, Role: domain.RoleUser,
			CreatedAt: now, Content: string(descriptorBytes), Source: "workflow",
		},
		Run:     domain.Run{ID: runID, SessionID: sessionID, Status: domain.RunAccepted, CreatedAt: now},
		Started: started, Prompt: &prompt,
	})
	if err != nil {
		t.Fatalf("admit workflow Run: %v", err)
	}
	workflowCtx := withRunPrompt(ctx, prompt)
	workflowCtx = withToolOperationCoordinator(workflowCtx, svc.newToolOperationCoordinator(runID, sessionID))
	workflowInput := orchestrationProofInput{
		A: nativeOrchestrationRequest{RunID: runID, RevisionDigest: revision, NodeKey: "a", Task: "alpha"},
		B: nativeOrchestrationRequest{RunID: runID, RevisionDigest: revision, NodeKey: "b", Task: "beta"},
	}
	withoutPromptCtx := withToolOperationCoordinator(ctx, svc.newToolOperationCoordinator(runID, sessionID))
	if _, err := svc.executeNativeOrchestrationWorkflowWith(withoutPromptCtx, workflowInput, nil); !errors.Is(err, ErrCheckpointPromptMismatch) {
		t.Fatalf("workflow without its prompt = %v, want prompt mismatch", err)
	}
	wrongPrompt := prompt
	wrongPrompt.RunID = "different-run"
	wrongPromptCtx := withToolOperationCoordinator(withRunPrompt(ctx, wrongPrompt), svc.newToolOperationCoordinator(runID, sessionID))
	if _, err := svc.executeNativeOrchestrationWorkflowWith(wrongPromptCtx, workflowInput, nil); !errors.Is(err, ErrCheckpointPromptMismatch) {
		t.Fatalf("workflow with another Run's prompt = %v, want prompt mismatch", err)
	}
	engine := svc.engine
	svc.engine = nil
	if _, err := svc.executeNativeOrchestrationWorkflowWith(workflowCtx, workflowInput, nil); err == nil {
		t.Fatal("workflow without its engine/checkpoint authority unexpectedly ran")
	}
	svc.engine = engine
	if _, err := backend.GetToolOperation(ctx, runID, workflowNodeOperationID(revision, "a")); !errors.Is(err, storage.ErrNotFound) {
		t.Fatalf("invalid workflow attempts admitted a node operation: %v", err)
	}

	entered := make(chan string, 2)
	release := make(chan struct{})
	nodeRunner := func(ctx context.Context, request nativeOrchestrationRequest) (nativeOrchestrationResult, error) {
		entered <- request.NodeKey
		select {
		case <-release:
		case <-ctx.Done():
			return nativeOrchestrationResult{}, ctx.Err()
		}
		return svc.runNativeOrchestration(ctx, request)
	}
	type invocation struct {
		output orchestrationProofOutput
		err    error
	}
	done := make(chan invocation, 1)
	go func() {
		output, err := svc.executeNativeOrchestrationWorkflowWith(workflowCtx, workflowInput, nodeRunner, compose.WithInterruptAfterNodes([]string{"a", "b"}))
		done <- invocation{output: output, err: err}
	}()

	// Both nodes must be runnable concurrently; a serial implementation would
	// time out here instead of silently satisfying the join.
	for range 2 {
		select {
		case <-entered:
		case <-time.After(5 * time.Second):
			close(release)
			t.Fatal("Eino Workflow did not overlap both independent nodes")
		}
	}
	close(release)
	result := <-done
	if _, interrupted := compose.ExtractInterruptInfo(result.err); !interrupted {
		t.Fatalf("workflow after-node interrupt = %v, want resumable Eino interrupt", result.err)
	}

	// Eino persists opaque workflow state at its interrupt boundary. Resume
	// with the same stable Run/checkpoint identity and consume both node outputs.
	if _, ok, err := checkpointStore.Get(workflowCtx, checkpointIDFor(runID)); err != nil || !ok {
		t.Fatalf("workflow checkpoint not readable under its prompt: present=%v err=%v", ok, err)
	}
	result.output, result.err = svc.executeNativeOrchestrationWorkflowWith(workflowCtx, workflowInput, nil, compose.WithInterruptAfterNodes([]string{"a", "b"}))
	if result.err != nil {
		t.Fatalf("resume native orchestration workflow: %v", result.err)
	}
	if result.output.Result != "alpha+beta" || result.output.Audit != "a:after-join" {
		t.Fatalf("resumed native orchestration result = %+v, want result=alpha+beta audit=a:after-join", result.output)
	}

	// Run and prompt identity bind Eino's opaque state to a unique workflow
	// checkpoint. Node operation events also retain this Run's stable scope.
	if checkpointIDFor(runID) == checkpointIDFor("another-run") {
		t.Fatal("different workflow Runs share a checkpoint identity")
	}
	beforeReplay := replayEvents(t, backend, runID)
	operationEventCount := 0
	for _, event := range beforeReplay {
		if event.Type == domain.EventToolOperation {
			operationEventCount++
		}
	}
	if operationEventCount != 6 {
		t.Fatalf("workflow journal has %d tool.operation transitions, want 6", operationEventCount)
	}
	replayed, err := svc.runNativeOrchestration(workflowCtx, workflowInput.A)
	if err != nil || replayed.Output != "alpha" {
		t.Fatalf("completed workflow node replay = %q/%v, want alpha without invocation", replayed.Output, err)
	}
	conflict := workflowInput.A
	conflict.Task = "changed task"
	if _, err := svc.runNativeOrchestration(workflowCtx, conflict); !errors.Is(err, storage.ErrToolOperationConflict) {
		t.Fatalf("same node key with changed task = %v, want operation conflict", err)
	}
	afterReplay := replayEvents(t, backend, runID)
	if len(afterReplay) != len(beforeReplay) {
		t.Fatalf("completed node replay appended events: before=%d after=%d", len(beforeReplay), len(afterReplay))
	}
	if _, err := svc.appendRunEvent(ctx, mapper.build(domain.EventRunCompleted, payloadRunCompleted{}), true); err != nil {
		t.Fatalf("complete workflow Run: %v", err)
	}
}

func TestOrchestrationApprovalResumesThroughService(t *testing.T) {
	fixture := newNativeOrchestrationApprovalFixture(t)
	ctx, svc, backend := fixture.ctx, fixture.svc, fixture.backend
	runID, sessionID, workflowCtx, workflowInput := fixture.runID, fixture.sessionID, fixture.workflowCtx, fixture.input

	bEntered := make(chan struct{})
	bReleased := make(chan struct{})
	var releaseOnce sync.Once
	releaseSibling := func() { releaseOnce.Do(func() { close(bReleased) }) }
	defer releaseSibling()
	aInterrupted := make(chan struct{})
	bCanceled := make(chan struct{}, 1)
	nodeRunner := func(nodeCtx context.Context, request nativeOrchestrationRequest) (nativeOrchestrationResult, error) {
		if request.NodeKey == "b" {
			close(bEntered)
			select {
			case <-bReleased:
			case <-nodeCtx.Done():
				bCanceled <- struct{}{}
				return nativeOrchestrationResult{}, nodeCtx.Err()
			}
		}
		output, runErr := svc.runNativeOrchestration(nodeCtx, request)
		if request.NodeKey == "a" && runErr != nil {
			close(aInterrupted)
		}
		return output, runErr
	}
	type invocation struct {
		output orchestrationProofOutput
		err    error
	}
	done := make(chan invocation, 1)
	go func() {
		output, invokeErr := svc.executeNativeOrchestrationWorkflowWith(workflowCtx, workflowInput, nodeRunner)
		done <- invocation{output: output, err: invokeErr}
	}()

	select {
	case <-bEntered:
	case <-time.After(5 * time.Second):
		releaseSibling()
		t.Fatal("independent workflow sibling did not start")
	}
	select {
	case <-aInterrupted:
	case <-time.After(5 * time.Second):
		releaseSibling()
		t.Fatal("approval-gated workflow node did not interrupt")
	}
	select {
	case <-bCanceled:
		releaseSibling()
		t.Fatal("approval interrupt cancelled its running sibling")
	case <-time.After(25 * time.Millisecond):
	}
	releaseSibling()
	var result invocation
	select {
	case result = <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("workflow did not return after its sibling completed")
	}
	if !errors.Is(result.err, ErrNativeOrchestrationApprovalPending) {
		t.Fatalf("workflow approval result = %v, want Service-managed pending approval", result.err)
	}
	approval := waitForPendingApproval(t, backend, runID)
	if approval.ResumeTarget == "" {
		t.Fatal("workflow approval has no durable resume target")
	}
	operationA := workflowNodeOperationID(workflowInput.A.RevisionDigest, "a")
	operationB := workflowNodeOperationID(workflowInput.A.RevisionDigest, "b")
	if op, err := backend.GetToolOperation(ctx, runID, operationA); err != nil || op.State != domain.ToolOperationAdmitted {
		t.Fatalf("approval-gated node operation = %+v/%v, want admitted but unclaimed", op, err)
	}
	if op, err := backend.GetToolOperation(ctx, runID, operationB); err != nil || op.State != domain.ToolOperationCompleted {
		t.Fatalf("sibling operation = %+v/%v, want completed before approval", op, err)
	}
	beforeResume := countWorkflowOperationEvents(t, backend, runID, operationB)
	if beforeResume != 3 {
		t.Fatalf("sibling operation transitions before approval = %d, want 3", beforeResume)
	}
	if err := svc.DecideApproval(ctx, approval.ID, domain.ApprovalApproved); err != nil {
		t.Fatalf("approve workflow node: %v", err)
	}
	waitForNativeOrchestrationRunStatus(t, backend, runID, domain.RunCompleted)
	if op, err := backend.GetToolOperation(ctx, runID, operationA); err != nil || op.State != domain.ToolOperationCompleted {
		t.Fatalf("approved node operation = %+v/%v, want completed", op, err)
	}
	if afterResume := countWorkflowOperationEvents(t, backend, runID, operationB); afterResume != beforeResume {
		t.Fatalf("sibling operation transitions after resume = %d, want unchanged %d", afterResume, beforeResume)
	}
	messages, err := backend.ListMessages(ctx, sessionID)
	if err != nil {
		t.Fatalf("list workflow output messages: %v", err)
	}
	foundOutput := false
	for _, message := range messages {
		if message.RunID == runID && message.Role == domain.RoleAssistant && message.Source == "workflow" {
			var output orchestrationProofOutput
			if err := json.Unmarshal([]byte(message.Content), &output); err != nil {
				t.Fatalf("decode workflow output: %v", err)
			}
			if output.Result != "alpha+beta" || output.Audit != "a:after-join" {
				t.Fatalf("workflow output = %+v, want both explicit outputs", output)
			}
			foundOutput = true
		}
	}
	if !foundOutput {
		t.Fatal("Service did not persist the resumed workflow output")
	}
}

func TestOrchestrationApprovalCancellationDoesNotClaimNode(t *testing.T) {
	fixture := newNativeOrchestrationApprovalFixture(t)
	svc, backend, runID := fixture.svc, fixture.backend, fixture.runID
	bEntered := make(chan struct{})
	bReleased := make(chan struct{})
	var releaseOnce sync.Once
	releaseSibling := func() { releaseOnce.Do(func() { close(bReleased) }) }
	defer releaseSibling()
	aInterrupted := make(chan struct{})
	bCanceled := make(chan struct{}, 1)
	type invocation struct{ err error }
	done := make(chan invocation, 1)
	nodeRunner := func(nodeCtx context.Context, request nativeOrchestrationRequest) (nativeOrchestrationResult, error) {
		if request.NodeKey == "b" {
			close(bEntered)
			select {
			case <-bReleased:
			case <-nodeCtx.Done():
				bCanceled <- struct{}{}
				return nativeOrchestrationResult{}, nodeCtx.Err()
			}
		}
		output, err := svc.runNativeOrchestration(nodeCtx, request)
		if request.NodeKey == "a" && err != nil {
			close(aInterrupted)
		}
		return output, err
	}
	go func() {
		_, err := svc.executeNativeOrchestrationWorkflowWith(fixture.workflowCtx, fixture.input, nodeRunner)
		done <- invocation{err: err}
	}()
	select {
	case <-bEntered:
	case <-time.After(5 * time.Second):
		releaseSibling()
		t.Fatal("independent workflow sibling did not start")
	}
	select {
	case <-aInterrupted:
	case <-time.After(5 * time.Second):
		releaseSibling()
		t.Fatal("approval-gated workflow node did not interrupt")
	}
	select {
	case <-bCanceled:
		releaseSibling()
		t.Fatal("approval interrupt cancelled its running sibling")
	case <-time.After(25 * time.Millisecond):
	}
	releaseSibling()
	select {
	case result := <-done:
		if !errors.Is(result.err, ErrNativeOrchestrationApprovalPending) {
			t.Fatalf("workflow result = %v, want pending approval", result.err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("workflow did not finish its sibling after approval interrupt")
	}
	approval := waitForPendingApproval(t, backend, runID)
	if !svc.Cancel(runID) {
		t.Fatal("cancel of a workflow pending on approval must report true")
	}
	waitForRunStatus(t, backend, runID, domain.RunCancelled)
	settled, err := backend.GetApproval(context.Background(), approval.ID)
	if err != nil || settled.Decision != domain.ApprovalCancelled {
		t.Fatalf("workflow approval after cancellation = %+v/%v, want cancelled", settled, err)
	}
	operationA := workflowNodeOperationID(fixture.input.A.RevisionDigest, "a")
	if op, err := backend.GetToolOperation(context.Background(), runID, operationA); err != nil || op.State != domain.ToolOperationAdmitted {
		t.Fatalf("cancelled approval-gated operation = %+v/%v, want admitted and unclaimed", op, err)
	}
}

func TestOrchestrationCancellationStopsApprovalSiblingAndLeavesNoPendingApproval(t *testing.T) {
	fixture := newNativeOrchestrationApprovalFixture(t)
	ctx, cancel := context.WithCancel(fixture.workflowCtx)
	defer cancel()
	bEntered := make(chan struct{})
	aInterrupted := make(chan struct{})
	bCanceled := make(chan struct{}, 1)
	type invocation struct{ err error }
	done := make(chan invocation, 1)
	nodeRunner := func(nodeCtx context.Context, request nativeOrchestrationRequest) (nativeOrchestrationResult, error) {
		if request.NodeKey == "b" {
			close(bEntered)
			<-nodeCtx.Done()
			bCanceled <- struct{}{}
			return nativeOrchestrationResult{}, nodeCtx.Err()
		}
		output, err := fixture.svc.runNativeOrchestration(nodeCtx, request)
		if request.NodeKey == "a" && err != nil {
			close(aInterrupted)
		}
		return output, err
	}
	go func() {
		_, err := fixture.svc.executeNativeOrchestrationWorkflowWith(ctx, fixture.input, nodeRunner)
		done <- invocation{err: err}
	}()
	select {
	case <-bEntered:
	case <-time.After(5 * time.Second):
		t.Fatal("independent workflow sibling did not start")
	}
	select {
	case <-aInterrupted:
	case <-time.After(5 * time.Second):
		t.Fatal("approval-gated workflow node did not interrupt")
	}
	cancel()
	select {
	case <-bCanceled:
	case <-time.After(5 * time.Second):
		t.Fatal("cancelling the workflow did not stop its running sibling")
	}
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("cancelled workflow did not return")
	}
	pending, err := fixture.backend.ListPendingApprovals(context.Background())
	if err != nil {
		t.Fatalf("list pending approvals: %v", err)
	}
	for _, approval := range pending {
		if approval.RunID == fixture.runID {
			t.Fatalf("cancelled workflow left approval pending: %+v", approval)
		}
	}
	operationA := workflowNodeOperationID(fixture.input.A.RevisionDigest, "a")
	if op, err := fixture.backend.GetToolOperation(context.Background(), fixture.runID, operationA); err != nil || op.State != domain.ToolOperationAdmitted {
		t.Fatalf("cancelled approval-gated operation = %+v/%v, want admitted and unclaimed", op, err)
	}
}

type nativeOrchestrationApprovalFixture struct {
	ctx         context.Context
	svc         *Service
	backend     *sqlite.Backend
	runID       domain.RunID
	sessionID   domain.SessionID
	workflowCtx context.Context
	input       orchestrationProofInput
}

func newNativeOrchestrationApprovalFixture(t *testing.T) nativeOrchestrationApprovalFixture {
	t.Helper()
	ctx := context.Background()
	svc, backend, _ := newTestService(t, testsupport.NewEchoModel())
	svc.deps.Approvals = backend
	svc.deps.ApprovalExpiration = 5 * time.Minute
	sessionID := domain.SessionID("session-native-orchestration-approval-" + newRunID())
	if err := backend.CreateSession(ctx, domain.Session{ID: sessionID, Title: "workflow approval", CreatedAt: time.Now().UnixMilli()}); err != nil {
		t.Fatalf("create session: %v", err)
	}
	svc.deps.Admission = backend
	svc.deps.GenerationID = "test-generation"
	runID := domain.RunID(newRunID())
	checkpointStore, err := NewVersionedCheckpointStore(backend.Blobs(), "v0.9.13")
	if err != nil {
		t.Fatalf("create checkpoint store: %v", err)
	}
	svc.engine.cfg.Checkpoints = checkpointStore
	descriptorBytes, revision, err := nativeOrchestrationDescriptorWithApproval("alpha", "beta", true)
	if err != nil {
		t.Fatalf("build workflow descriptor: %v", err)
	}
	prompt, err := buildPromptSnapshot(PromptInput{
		RunID: runID, GenerationID: svc.deps.GenerationID,
		Capture: maskcontract.Capture{Selection: maskcontract.Selection{SessionID: sessionID}},
	})
	if err != nil {
		t.Fatalf("build prompt snapshot: %v", err)
	}
	mapper := newEventMapper(runID, 64<<10)
	started := mapper.build(domain.EventRunStarted, payloadRunStarted{
		Provider: "test", Model: "test-model", Mode: string(domain.RunModeNormal), Face: string(domain.FaceWeb),
		PolicyProfile: string(domain.PolicyProfileDefault), PolicyHash: "test-policy",
		PromptSchema: prompt.SchemaVersion, PromptDigest: prompt.PayloadSHA256,
	})
	_, err = backend.CommitRunAdmission(ctx, storage.RunAdmission{
		Message: domain.Message{ID: newMessageID(), SessionID: sessionID, RunID: runID, Role: domain.RoleUser,
			CreatedAt: time.Now().UnixMilli(), Content: string(descriptorBytes), Source: "workflow"},
		Run:     domain.Run{ID: runID, SessionID: sessionID, Status: domain.RunAccepted, CreatedAt: time.Now().UnixMilli()},
		Started: started, Prompt: &prompt,
	})
	if err != nil {
		t.Fatalf("admit workflow Run: %v", err)
	}
	workflowCtx := withRunPrompt(ctx, prompt)
	workflowCtx = withWorkspaceID(withSessionID(withRunID(workflowCtx, runID), sessionID), "")
	workflowCtx = withPolicySnapshot(withPolicyProfile(withRunMode(withFace(withSelectedTools(workflowCtx, nil), domain.FaceWeb), domain.RunModeNormal), domain.PolicyProfileDefault), domain.PolicySnapshot{Hash: "test-policy"})
	workflowCtx = withSessionSandbox(workflowCtx, domain.SandboxModeWorkspaceWrite, domain.ApprovalPolicyAsk)
	workflowCtx = withToolOperationCoordinator(workflowCtx, svc.newToolOperationCoordinator(runID, sessionID))
	input := orchestrationProofInput{
		A: nativeOrchestrationRequest{RunID: runID, RevisionDigest: revision, NodeKey: "a", Task: "alpha", RequiresApproval: true},
		B: nativeOrchestrationRequest{RunID: runID, RevisionDigest: revision, NodeKey: "b", Task: "beta"},
	}
	return nativeOrchestrationApprovalFixture{ctx: ctx, svc: svc, backend: backend, runID: runID, sessionID: sessionID, workflowCtx: workflowCtx, input: input}
}

func waitForNativeOrchestrationRunStatus(t *testing.T, backend *sqlite.Backend, runID domain.RunID, want domain.RunStatus) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	var last domain.RunStatus
	for time.Now().Before(deadline) {
		run, err := backend.GetRun(context.Background(), runID)
		if err == nil {
			last = run.Status
			if run.Status == want {
				return
			}
			if run.Status.Terminal() {
				events := replayEvents(t, backend, runID)
				if len(events) != 0 {
					t.Fatalf("workflow resume reached %s instead of %s: %s %s", run.Status, want, events[len(events)-1].Type, string(events[len(events)-1].Payload))
				}
				t.Fatalf("workflow resume reached %s instead of %s", run.Status, want)
			}
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("workflow run %s never reached %s (last status %s)", runID, want, last)
}

func countWorkflowOperationEvents(t *testing.T, backend *sqlite.Backend, runID domain.RunID, operationID string) int {
	t.Helper()
	count := 0
	for _, event := range replayEvents(t, backend, runID) {
		if event.Type != domain.EventToolOperation {
			continue
		}
		var payload struct {
			OperationID string `json:"operation_id"`
		}
		if err := json.Unmarshal(event.Payload, &payload); err != nil {
			t.Fatalf("decode tool operation event: %v", err)
		}
		if payload.OperationID == operationID {
			count++
		}
	}
	return count
}

func replayEvents(t *testing.T, backend *sqlite.Backend, runID domain.RunID) []domain.RunEvent {
	t.Helper()
	iterator, err := backend.Replay(context.Background(), runID, 0)
	if err != nil {
		t.Fatalf("replay workflow Run: %v", err)
	}
	defer func() { _ = iterator.Close() }()
	var events []domain.RunEvent
	for iterator.Next() {
		events = append(events, iterator.Value().Event)
	}
	if err := iterator.Err(); err != nil {
		t.Fatalf("read workflow Journal: %v", err)
	}
	return events
}
