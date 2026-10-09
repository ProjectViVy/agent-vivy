package runtime

import (
	"context"
	"errors"
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
