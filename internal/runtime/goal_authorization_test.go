package runtime

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/cloudwego/eino/schema"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/storage/sqlite"
	"agent-vivy/internal/tools"
)

func TestCreateGoalNeverRequiresHumanApprovalBeforeDurableGoalOrWake(t *testing.T) {
	for _, decision := range []string{domain.ApprovalDenied, domain.ApprovalApproved} {
		t.Run(decision, func(t *testing.T) {
			ctx := context.Background()
			backend, err := sqlite.Open(ctx, filepath.Join(t.TempDir(), "goal-authorization.db"))
			if err != nil {
				t.Fatalf("open sqlite: %v", err)
			}
			t.Cleanup(func() { _ = backend.Close() })

			const sessionID = domain.SessionID("sess-create-goal-never")
			if err := backend.CreateSession(ctx, domain.Session{ID: sessionID, Title: "Goal approval", CreatedAt: 1,
				SandboxMode: string(domain.SandboxModeWorkspaceWrite), ApprovalPolicy: string(domain.ApprovalPolicyNever)}); err != nil {
				t.Fatalf("create session: %v", err)
			}
			toolset, err := tools.NewRegistry(tools.NewCreateGoal()).Resolve([]string{tools.CreateGoalName})
			if err != nil {
				t.Fatalf("resolve tools: %v", err)
			}
			model := &recordingChatModel{inner: NewScriptedModel(
				schema.AssistantMessage("", []schema.ToolCall{{ID: "create-goal-call", Function: schema.FunctionCall{
					Name: tools.CreateGoalName, Arguments: `{"objective":"ship the reviewed change","max_rounds":1}`,
				}}}),
				schema.AssistantMessage("The Goal request was decided.", nil),
				schema.AssistantMessage("The one admitted Goal round ended.", nil),
			)}
			checkpoints, err := NewVersionedCheckpointStore(backend.Blobs(), "goal-authorization-test")
			if err != nil {
				t.Fatalf("checkpoint store: %v", err)
			}
			engine, err := NewEngine(ctx, model, toolset, EngineConfig{
				StreamBuffer: 8, MaxEventPayloadBytes: 64 << 10, Checkpoints: checkpoints,
			})
			if err != nil {
				t.Fatalf("new engine: %v", err)
			}
			svc := NewService(engine, "scripted", "scripted-v0", ServiceDeps{
				Journal: backend, Work: backend, Runs: backend, Messages: backend, Sessions: backend,
				PrimaryRuns: backend, GoalRuns: backend, Approvals: backend, Sink: newTestSink(),
				ApprovalExpiration: 5 * time.Minute, PolicyDefaultProfile: domain.PolicyProfileFullAuto,
			})
			runID, err := svc.Run(ctx, sessionID, "Create a bounded Goal.")
			if err != nil {
				t.Fatalf("start run: %v", err)
			}

			approval := waitForCreateGoalApproval(t, backend, runID, sessionID, model)
			if approval.ToolName != tools.CreateGoalName || approval.ToolCallID != "create-goal-call" {
				t.Fatalf("pending approval identity = %s/%s, want create_goal/create-goal-call", approval.ToolName, approval.ToolCallID)
			}
			if count := modelInputCount(model); count != 1 {
				t.Fatalf("model requests before human decision = %d, want 1", count)
			}
			work, err := backend.ReadWork(ctx, sessionID)
			if err != nil {
				t.Fatalf("read pending work: %v", err)
			}
			if work.Goal != nil {
				t.Fatalf("pending approval already created a Goal: %+v", work.Goal)
			}
			if hasWorkKind(t, backend, sessionID, domain.WorkEventGoalCreated, domain.WorkEventGoalRoundAdmitted) {
				t.Fatal("pending approval persisted Goal creation or admission work")
			}

			if err := svc.DecideApproval(ctx, approval.ID, decision); err != nil {
				t.Fatalf("decide create_goal: %v", err)
			}
			waitForRunStatus(t, backend, runID, domain.RunCompleted)
			work, err = backend.ReadWork(ctx, sessionID)
			if err != nil {
				t.Fatalf("read work after decision: %v", err)
			}
			if decision == domain.ApprovalDenied {
				if work.Goal != nil {
					t.Fatalf("denied create_goal left a durable Goal: %+v", work.Goal)
				}
				if hasWorkKind(t, backend, sessionID, domain.WorkEventGoalCreated, domain.WorkEventGoalRoundAdmitted) {
					t.Fatal("denied create_goal persisted Goal creation or admission work")
				}
				if count := modelInputCount(model); count != 2 {
					t.Fatalf("model requests after denial = %d, want one resumed decision cycle", count)
				}
				return
			}

			if work.Goal == nil || work.Goal.Objective != "ship the reviewed change" {
				t.Fatalf("approved create_goal state = %+v, want the authorized Goal", work.Goal)
			}
			waitForWorkKind(t, backend, sessionID, domain.WorkEventGoalRoundAdmitted)
			waitForModelInputCount(t, model, 3)
			waitForGoalRunCompletion(t, backend, sessionID, runID)
		})
	}
}

func waitForCreateGoalApproval(t *testing.T, backend *sqlite.Backend, runID domain.RunID, sessionID domain.SessionID, model *recordingChatModel) domain.Approval {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		pending, err := backend.ListPendingApprovals(context.Background())
		if err != nil {
			t.Fatalf("list pending approvals: %v", err)
		}
		for _, approval := range pending {
			if approval.RunID == runID {
				return approval
			}
		}
		work, err := backend.ReadWork(context.Background(), sessionID)
		if err != nil {
			t.Fatalf("read work while awaiting approval: %v", err)
		}
		if work.Goal != nil {
			t.Fatalf("Goal became durable before human approval: %+v", work.Goal)
		}
		run, err := backend.GetRun(context.Background(), runID)
		if err != nil {
			t.Fatalf("read run while awaiting approval: %v", err)
		}
		if run.Status.Terminal() {
			var eventDetails []string
			for _, event := range replayAll(t, backend, runID) {
				eventDetails = append(eventDetails, string(event.Type)+":"+string(event.Payload))
			}
			t.Fatalf("run terminated before human approval: %s; Goal=%+v events=%v model_requests=%d", run.Status, work.Goal, eventDetails, modelInputCount(model))
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("create_goal under approval policy never did not suspend for human approval")
	return domain.Approval{}
}

func hasWorkKind(t *testing.T, backend *sqlite.Backend, sessionID domain.SessionID, kinds ...domain.WorkEventKind) bool {
	t.Helper()
	events, _, err := backend.ReplayWork(context.Background(), sessionID, domain.WorkState{SessionID: sessionID}, 100)
	if err != nil {
		t.Fatalf("replay work: %v", err)
	}
	for _, event := range events {
		for _, kind := range kinds {
			if event.Kind == kind {
				return true
			}
		}
	}
	return false
}

func waitForWorkKind(t *testing.T, backend *sqlite.Backend, sessionID domain.SessionID, kind domain.WorkEventKind) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if hasWorkKind(t, backend, sessionID, kind) {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("work event %s was not committed", kind)
}

func modelInputCount(model *recordingChatModel) int {
	model.mu.Lock()
	defer model.mu.Unlock()
	return len(model.inputs)
}

func waitForModelInputCount(t *testing.T, model *recordingChatModel, want int) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if got := modelInputCount(model); got >= want {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("model requests = %d, want at least %d", modelInputCount(model), want)
}

func waitForGoalRunCompletion(t *testing.T, backend *sqlite.Backend, sessionID domain.SessionID, primaryRunID domain.RunID) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		runs, err := backend.ListRunsBySession(context.Background(), sessionID)
		if err != nil {
			t.Fatalf("list runs after Goal approval: %v", err)
		}
		for _, run := range runs {
			if run.ID == primaryRunID {
				continue
			}
			if run.Status == domain.RunCompleted {
				return
			}
			if run.Status.Terminal() {
				t.Fatalf("approved Goal run %s ended as %s", run.ID, run.Status)
			}
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("approved Goal run did not complete")
}
