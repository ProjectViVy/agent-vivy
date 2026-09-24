package runtime_test

import (
	"context"
	"encoding/json"
	"path/filepath"
	"testing"
	"time"

	"github.com/cloudwego/eino/schema"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/events"
	"agent-vivy/internal/rpc"
	"agent-vivy/internal/runtime"
	"agent-vivy/internal/storage/sqlite"
	"agent-vivy/internal/tools"
)

func TestStartGoalDecisionRPCWakesExactlyOneRound(t *testing.T) {
	ctx := context.Background()
	backend, err := sqlite.Open(ctx, filepath.Join(t.TempDir(), "plan-goal-rpc.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = backend.Close() })
	const sessionID domain.SessionID = "sess-plan-goal-rpc"
	if err := backend.CreateSession(ctx, domain.Session{
		ID: sessionID, CreatedAt: 1,
		SandboxMode: string(domain.SandboxModeWorkspaceWrite), ApprovalPolicy: string(domain.ApprovalPolicyAuto),
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := backend.CommitWork(ctx, domain.WorkMutation{
		SessionID: sessionID, RequestID: "enter", RequestHash: "enter", Kind: domain.WorkEventPlanEntered,
	}); err != nil {
		t.Fatal(err)
	}
	toolset, err := tools.NewRegistry(tools.NewSubmitPlan()).Resolve([]string{tools.SubmitPlanName})
	if err != nil {
		t.Fatal(err)
	}
	checkpoints, err := runtime.NewVersionedCheckpointStore(backend.Blobs(), "plan-goal-rpc")
	if err != nil {
		t.Fatal(err)
	}
	engine, err := runtime.NewEngine(ctx, runtime.NewScriptedModel(
		schema.AssistantMessage("", []schema.ToolCall{{ID: "submit", Function: schema.FunctionCall{Name: tools.SubmitPlanName, Arguments: `{"markdown":"1. inspect\n2. implement"}`}}}),
		schema.AssistantMessage("Proceed with the plan.", nil),
		schema.AssistantMessage("Goal round complete.", nil),
	), toolset, runtime.EngineConfig{
		StreamBuffer: 8, MaxEventPayloadBytes: 64 << 10, Checkpoints: checkpoints,
		AutoApproveTools: []string{tools.SubmitPlanName},
	})
	if err != nil {
		t.Fatal(err)
	}
	bus := events.NewBus(8)
	svc := runtime.NewService(engine, "scripted", "scripted-v0", runtime.ServiceDeps{
		Journal: backend, Runs: backend, Messages: backend, Sessions: backend,
		PrimaryRuns: backend, GoalRuns: backend, Work: backend, Approvals: backend, Questions: backend, Sink: bus,
	})
	t.Cleanup(func() {
		svc.StopAutomaticWork()
		svc.CancelAll()
		idleCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if !svc.WaitIdle(idleCtx) {
			t.Error("service did not become idle")
		}
	})
	handler, err := rpc.NewControlHandler(rpc.ControlDeps{
		Sessions: backend, Messages: backend, Runs: backend, Journal: backend, Work: backend,
		Approvals: backend, Questions: backend, Bus: bus, Service: svc,
	})
	if err != nil {
		t.Fatal(err)
	}
	origin, err := svc.Run(ctx, sessionID, "review a Plan")
	if err != nil {
		t.Fatal(err)
	}
	var pending domain.WorkState
	deadline := time.Now().Add(5 * time.Second)
	for {
		pending, err = backend.ReadWork(ctx, sessionID)
		if err != nil {
			t.Fatal(err)
		}
		if pending.Plan.ReviewStatus == domain.PlanReviewPending && pending.Plan.ResumeTarget != "" {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("Plan review did not suspend: %+v", pending.Plan)
		}
		time.Sleep(5 * time.Millisecond)
	}
	params, err := json.Marshal(map[string]any{
		"session_id": sessionID, "expected_version": pending.Version, "request_id": "decide-rpc",
		"submission_id": pending.Plan.SubmissionID, "action": "start_goal",
		"goal_id": "reviewed-goal", "goal_revision": 1,
		"objective": "complete reviewed plan", "max_rounds": 1,
	})
	if err != nil {
		t.Fatal(err)
	}
	request := rpc.Request{JSONRPC: "2.0", Method: "plan/decide", Params: params}
	first, rpcErr := handler.Handle(ctx, nil, request)
	if rpcErr != nil {
		t.Fatalf("first plan/decide: %v", rpcErr)
	}
	replayed, rpcErr := handler.Handle(ctx, nil, request)
	if rpcErr != nil {
		t.Fatalf("replayed plan/decide: %v", rpcErr)
	}
	firstJSON, _ := json.Marshal(first)
	replayJSON, _ := json.Marshal(replayed)
	var firstView, replayView struct {
		Event struct {
			Seq int64 `json:"seq"`
		} `json:"event"`
		Replayed bool `json:"replayed"`
	}
	if err := json.Unmarshal(firstJSON, &firstView); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(replayJSON, &replayView); err != nil {
		t.Fatal(err)
	}
	if firstView.Replayed || !replayView.Replayed || firstView.Event.Seq != replayView.Event.Seq {
		t.Fatalf("decision replay = %s / %s", firstJSON, replayJSON)
	}
	deadline = time.Now().Add(5 * time.Second)
	for {
		originRun, runErr := backend.GetRun(ctx, origin)
		state, workErr := backend.ReadWork(ctx, sessionID)
		if runErr != nil || workErr != nil {
			t.Fatalf("origin/Work = %v / %v", runErr, workErr)
		}
		if originRun.Status == domain.RunCompleted && state.Goal != nil && state.Goal.RoundsStarted == 1 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("RPC decision did not admit Goal after origin terminal: origin=%s Work=%+v", originRun.Status, state)
		}
		time.Sleep(5 * time.Millisecond)
	}
	events, _, err := backend.ReplayWork(ctx, sessionID, domain.WorkState{SessionID: sessionID}, 20)
	if err != nil {
		t.Fatal(err)
	}
	decisions, admissions := 0, 0
	for _, event := range events {
		if event.Kind == domain.WorkEventPlanDecided {
			decisions++
		}
		if event.Kind == domain.WorkEventGoalRoundAdmitted {
			admissions++
		}
	}
	if decisions != 1 || admissions != 1 {
		t.Fatalf("Journal decisions/admissions = %d/%d, want 1/1", decisions, admissions)
	}
}
