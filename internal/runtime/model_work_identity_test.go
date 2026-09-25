package runtime

import (
	"context"
	"errors"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/cloudwego/eino/schema"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/storage"
	"agent-vivy/internal/storage/sqlite"
	"agent-vivy/internal/tools"
)

type recordingWorkStore struct {
	storage.WorkStore
	mu        sync.Mutex
	mutations []domain.WorkMutation
}

func (s *recordingWorkStore) CommitWork(ctx context.Context, mutation domain.WorkMutation) (storage.WorkCommitResult, error) {
	s.mu.Lock()
	s.mutations = append(s.mutations, mutation)
	s.mu.Unlock()
	return s.WorkStore.CommitWork(ctx, mutation)
}

func (s *recordingWorkStore) snapshot() []domain.WorkMutation {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]domain.WorkMutation(nil), s.mutations...)
}

func TestModelWorkIdentityUsesEinoCallIDForDistinctCallsAndRetries(t *testing.T) {
	ctx := context.Background()
	backend, err := sqlite.Open(ctx, filepath.Join(t.TempDir(), "model-work-identity.db"))
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { _ = backend.Close() })
	const sessionID = domain.SessionID("sess-model-work-identity")
	if err := backend.CreateSession(ctx, domain.Session{ID: sessionID, Title: "Model work identity", CreatedAt: 1,
		SandboxMode: string(domain.SandboxModeWorkspaceWrite), ApprovalPolicy: string(domain.ApprovalPolicyAuto)}); err != nil {
		t.Fatalf("create session: %v", err)
	}
	toolset, err := tools.NewRegistry(tools.NewEnterPlanMode()).Resolve([]string{tools.EnterPlanModeName})
	if err != nil {
		t.Fatalf("resolve tools: %v", err)
	}
	model := NewScriptedModel(
		schema.AssistantMessage("", []schema.ToolCall{{ID: "retry-call", Function: schema.FunctionCall{Name: tools.EnterPlanModeName, Arguments: `{}`}}}),
		schema.AssistantMessage("", []schema.ToolCall{{ID: "retry-call", Function: schema.FunctionCall{Name: tools.EnterPlanModeName, Arguments: `{}`}}}),
		schema.AssistantMessage("", []schema.ToolCall{{ID: "distinct-call", Function: schema.FunctionCall{Name: tools.EnterPlanModeName, Arguments: `{}`}}}),
		schema.AssistantMessage("Plan mode is active.", nil),
	)
	engine, err := NewEngine(ctx, model, toolset, EngineConfig{StreamBuffer: 8, MaxEventPayloadBytes: 64 << 10})
	if err != nil {
		t.Fatalf("new engine: %v", err)
	}
	work := &recordingWorkStore{WorkStore: backend}
	svc := NewService(engine, "scripted", "scripted-v0", ServiceDeps{
		Journal: backend, Work: work, Runs: backend, Messages: backend, Sessions: backend,
		PrimaryRuns: backend, Sink: newTestSink(), PolicyDefaultProfile: domain.PolicyProfileFullAuto,
	})
	runID, err := svc.Run(ctx, sessionID, "Enter Plan mode.")
	if err != nil {
		t.Fatalf("start run: %v", err)
	}
	waitForTerminalRun(t, backend, runID)

	mutations := work.snapshot()
	if len(mutations) != 3 {
		t.Fatalf("model work mutations = %d, want one per Eino call (including the retry), got %+v", len(mutations), mutations)
	}
	if mutations[0].RequestID != mutations[1].RequestID || mutations[0].RequestHash != mutations[1].RequestHash {
		t.Fatalf("exact same-call retry changed request identity: first=%+v retry=%+v", mutations[0], mutations[1])
	}
	if mutations[2].RequestID == mutations[0].RequestID {
		t.Fatalf("distinct Eino call ID reused request identity: retry=%+v distinct=%+v", mutations[0], mutations[2])
	}
}

func TestModelWorkIdentityChangedArgsConflictAtWorkStore(t *testing.T) {
	ctx := context.Background()
	backend, err := sqlite.Open(ctx, filepath.Join(t.TempDir(), "model-work-conflict.db"))
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { _ = backend.Close() })
	const sessionID = domain.SessionID("sess-model-work-conflict")
	if err := backend.CreateSession(ctx, domain.Session{ID: sessionID, Title: "Model work conflict", CreatedAt: 1}); err != nil {
		t.Fatalf("create session: %v", err)
	}

	requestID, requestHash, err := modelWorkIdentity("run-model-1", "create-goal", "call-reused", map[string]string{"objective": "first"})
	if err != nil {
		t.Fatalf("initial model identity: %v", err)
	}
	first, err := backend.CommitWork(ctx, domain.WorkMutation{
		SessionID: sessionID, ExpectedVersion: 0, RequestID: requestID, RequestHash: requestHash,
		Kind: domain.WorkEventGoalCreated, Goal: domain.GoalRef{ID: "goal-created", Revision: 1},
		Objective: "first", MaxRounds: 2,
	})
	if err != nil {
		t.Fatalf("commit initial request: %v", err)
	}
	retry, err := backend.CommitWork(ctx, domain.WorkMutation{
		SessionID: sessionID, ExpectedVersion: 0, RequestID: requestID, RequestHash: requestHash,
		Kind: domain.WorkEventGoalCreated, Goal: domain.GoalRef{ID: "goal-created", Revision: 1},
		Objective: "first", MaxRounds: 2,
	})
	if err != nil || !retry.Replayed || retry.Event.Seq != first.Event.Seq {
		t.Fatalf("exact same-call retry = %+v / %v, want original committed result", retry, err)
	}
	changedID, changedHash, err := modelWorkIdentity("run-model-1", "create-goal", "call-reused", map[string]string{"objective": "changed"})
	if err != nil {
		t.Fatalf("changed model identity: %v", err)
	}
	if changedID != requestID || changedHash == requestHash {
		t.Fatalf("same call ID changed identity incorrectly: original=(%q,%q) changed=(%q,%q)", requestID, requestHash, changedID, changedHash)
	}
	_, err = backend.CommitWork(ctx, domain.WorkMutation{
		SessionID: sessionID, ExpectedVersion: first.State.Version, RequestID: changedID, RequestHash: changedHash,
		Kind: domain.WorkEventGoalCreated, Goal: domain.GoalRef{ID: "goal-created", Revision: 1},
		Objective: "changed", MaxRounds: 2,
	})
	if !errors.Is(err, storage.ErrWorkRequestConflict) {
		t.Fatalf("same tool-call ID with changed args = %v, want ErrWorkRequestConflict", err)
	}
}

func waitForTerminalRun(t *testing.T, backend *sqlite.Backend, runID domain.RunID) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		run, err := backend.GetRun(context.Background(), runID)
		if err != nil {
			t.Fatalf("get run: %v", err)
		}
		if run.Status.Terminal() {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("run did not reach a terminal state")
}
