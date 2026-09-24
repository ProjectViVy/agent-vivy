package runtime_test

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"

	"agent-vivy/internal/actionhost"
	"agent-vivy/internal/domain"
	"agent-vivy/internal/events"
	"agent-vivy/internal/rpc"
	"agent-vivy/internal/runtime"
	"agent-vivy/internal/storage"
	"agent-vivy/internal/storage/sqlite"
	"agent-vivy/internal/tools"
)

type heldRPCPlanModel struct {
	inner         model.ToolCallingChatModel
	calls         atomic.Int32
	resumeEntered chan struct{}
	release       chan struct{}
}

func (m *heldRPCPlanModel) Stream(ctx context.Context, messages []*schema.Message, opts ...model.Option) (*schema.StreamReader[*schema.Message], error) {
	if m.calls.Add(1) == 2 {
		close(m.resumeEntered)
		select {
		case <-m.release:
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	return m.inner.Stream(ctx, messages, opts...)
}

func (m *heldRPCPlanModel) Generate(ctx context.Context, messages []*schema.Message, opts ...model.Option) (*schema.Message, error) {
	return m.inner.Generate(ctx, messages, opts...)
}

func (m *heldRPCPlanModel) WithTools(infos []*schema.ToolInfo) (model.ToolCallingChatModel, error) {
	inner, err := m.inner.WithTools(infos)
	if err != nil {
		return nil, err
	}
	m.inner = inner
	return m, nil
}

type goalWakeReadProbe struct {
	storage.WorkStore
	sawActiveGoal chan struct{}
	once          sync.Once
}

func (p *goalWakeReadProbe) ReadWork(ctx context.Context, sessionID domain.SessionID) (domain.WorkState, error) {
	state, err := p.WorkStore.ReadWork(ctx, sessionID)
	if err == nil && state.Goal != nil && state.Goal.Phase == domain.WorkPhaseActive && !state.Plan.Active {
		p.once.Do(func() { close(p.sawActiveGoal) })
	}
	return state, err
}

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
	const otherSessionID domain.SessionID = "sess-plan-goal-rpc-other"
	if err := backend.CreateSession(ctx, domain.Session{ID: otherSessionID, CreatedAt: 1}); err != nil {
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
	chat := &heldRPCPlanModel{inner: runtime.NewScriptedModel(
		schema.AssistantMessage("", []schema.ToolCall{{ID: "submit", Function: schema.FunctionCall{Name: tools.SubmitPlanName, Arguments: `{"markdown":"1. inspect\n2. implement"}`}}}),
		schema.AssistantMessage("Proceed with the plan.", nil),
		schema.AssistantMessage("Goal round complete.", nil),
	), resumeEntered: make(chan struct{}), release: make(chan struct{})}
	var releaseOnce sync.Once
	release := func() { releaseOnce.Do(func() { close(chat.release) }) }
	t.Cleanup(release)
	engine, err := runtime.NewEngine(ctx, chat, toolset, runtime.EngineConfig{
		StreamBuffer: 8, MaxEventPayloadBytes: 64 << 10, Checkpoints: checkpoints,
		AutoApproveTools: []string{tools.SubmitPlanName},
	})
	if err != nil {
		t.Fatal(err)
	}
	bus := events.NewBus(8)
	probe := &goalWakeReadProbe{WorkStore: backend, sawActiveGoal: make(chan struct{})}
	svc := runtime.NewService(engine, "scripted", "scripted-v0", runtime.ServiceDeps{
		Journal: backend, Runs: backend, Messages: backend, Sessions: backend,
		PrimaryRuns: backend, GoalRuns: backend, Work: probe, Approvals: backend, Questions: backend, Sink: bus,
	})
	t.Cleanup(func() {
		release()
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
	left, right := net.Pipe()
	peerCtx, stopPeers := context.WithCancel(ctx)
	server := rpc.NewPeer(rpc.NewJSONLTransport(left, left, left.Close), handler, rpc.Options{
		OutgoingBuffer: 8, Identity: actionhost.Identity{ID: "face/connection", Face: "web"},
	})
	client := rpc.NewPeer(rpc.NewJSONLTransport(right, right, right.Close), nil, rpc.Options{OutgoingBuffer: 8})
	go func() { _ = server.Serve(peerCtx) }()
	go func() { _ = client.Serve(peerCtx) }()
	t.Cleanup(func() { stopPeers(); _ = left.Close(); _ = right.Close() })
	bound, err := client.Call(ctx, "session/get", map[string]any{"session_id": sessionID})
	if err != nil || len(bound) == 0 {
		t.Fatalf("bind authenticated session: %s / %v", bound, err)
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
	params := map[string]any{
		"session_id": sessionID, "expected_version": pending.Version, "request_id": "decide-rpc",
		"submission_id": pending.Plan.SubmissionID, "action": "start_goal",
		"goal_id": "reviewed-goal", "goal_revision": 1,
		"objective": "complete reviewed plan", "max_rounds": 1,
	}
	wrongParams := make(map[string]any, len(params))
	for key, value := range params {
		wrongParams[key] = value
	}
	wrongParams["session_id"] = otherSessionID
	_, err = client.Call(ctx, "plan/decide", wrongParams)
	var rejected *rpc.Error
	if !errors.As(err, &rejected) || rejected.Code != rpc.CodeConflict {
		t.Fatalf("mismatched authenticated plan/decide = %v, want session conflict", err)
	}
	other, err := backend.ReadWork(ctx, otherSessionID)
	if err != nil || other.Version != 0 {
		t.Fatalf("mismatched session Work = %+v / %v", other, err)
	}
	firstJSON, err := client.Call(ctx, "plan/decide", params)
	if err != nil {
		t.Fatalf("first authenticated plan/decide: %v", err)
	}
	replayJSON, err := client.Call(ctx, "plan/decide", params)
	if err != nil {
		t.Fatalf("replayed authenticated plan/decide: %v", err)
	}
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
	select {
	case <-chat.resumeEntered:
	case <-time.After(5 * time.Second):
		t.Fatal("origin did not resume after RPC decision")
	}
	select {
	case <-probe.sawActiveGoal:
	case <-time.After(5 * time.Second):
		t.Fatal("initial RPC decision did not wake Goal before origin terminal cleanup")
	}
	originBeforeRelease, err := backend.GetRun(ctx, origin)
	if err != nil || originBeforeRelease.Status != domain.RunActive {
		t.Fatalf("origin reached terminal before observed RPC wake: %+v / %v", originBeforeRelease, err)
	}
	release()
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
