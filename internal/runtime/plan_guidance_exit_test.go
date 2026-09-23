package runtime

import (
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cloudwego/eino/schema"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/storage"
	"agent-vivy/internal/storage/sqlite"
	"agent-vivy/internal/tools"
)

type leavePlanAfterEchoTool struct {
	inner     tools.Tool
	work      storage.WorkStore
	sessionID domain.SessionID
}

func (t leavePlanAfterEchoTool) Spec() domain.ToolSpec { return t.inner.Spec() }

func (t leavePlanAfterEchoTool) InvokableRun(ctx context.Context, args json.RawMessage) (string, error) {
	result, err := t.inner.InvokableRun(ctx, args)
	if err != nil {
		return result, err
	}
	state, err := t.work.ReadWork(ctx, t.sessionID)
	if err != nil {
		return "", err
	}
	_, err = t.work.CommitWork(ctx, domain.WorkMutation{
		SessionID: t.sessionID, ExpectedVersion: state.Version,
		RequestID: "scripted-leave-plan", RequestHash: "scripted-leave-plan",
		Kind: domain.WorkEventPlanLeft,
	})
	return result, err
}

func TestPlanGuidanceRemovedAfterExitOnScriptedServicePath(t *testing.T) {
	ctx := context.Background()
	backend, err := sqlite.Open(ctx, filepath.Join(t.TempDir(), "plan-guidance-exit.db"))
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { _ = backend.Close() })
	const sessionID = domain.SessionID("sess-plan-guidance-exit")
	if err := backend.CreateSession(ctx, domain.Session{
		ID: sessionID, Title: "Plan guidance exit", CreatedAt: 1,
		SandboxMode: string(domain.SandboxModeWorkspaceWrite), ApprovalPolicy: string(domain.ApprovalPolicyAuto),
	}); err != nil {
		t.Fatalf("create session: %v", err)
	}
	if _, err := backend.CommitWork(ctx, domain.WorkMutation{
		SessionID: sessionID, ExpectedVersion: 0, RequestID: "seed-active-plan", RequestHash: "seed-active-plan",
		Kind: domain.WorkEventPlanEntered,
	}); err != nil {
		t.Fatalf("seed active Plan: %v", err)
	}
	tool := leavePlanAfterEchoTool{inner: tools.NewEchoInfo(), work: backend, sessionID: sessionID}
	toolset, err := tools.NewRegistry(tool).Resolve([]string{tools.EchoInfoName})
	if err != nil {
		t.Fatalf("resolve test tool: %v", err)
	}
	script := &recordingChatModel{inner: NewScriptedModel(
		schema.AssistantMessage("", []schema.ToolCall{{
			ID: "leave-plan", Function: schema.FunctionCall{Name: tools.EchoInfoName, Arguments: `{"text":"leave Plan"}`},
		}}),
		schema.AssistantMessage("Plan has ended.", nil),
	)}
	engine, err := NewEngine(ctx, script, toolset, EngineConfig{StreamBuffer: 8, MaxEventPayloadBytes: 64 << 10})
	if err != nil {
		t.Fatalf("new engine: %v", err)
	}
	service := NewService(engine, "scripted", "scripted-v0", ServiceDeps{
		Journal: backend, Runs: backend, Messages: backend, Sessions: backend,
		PrimaryRuns: backend, Work: backend, Sink: newTestSink(),
	})
	runID, err := service.RunWithOptions(ctx, sessionID, "Continue this plan.", RunOptions{
		CollaborationMode: domain.CollaborationModePlan, CollaborationVersion: domain.CollaborationVersion,
	})
	if err != nil {
		t.Fatalf("run in active Plan: %v", err)
	}
	waitForRunStatus(t, backend, runID, domain.RunCompleted)

	script.mu.Lock()
	inputs := append([][]*schema.Message(nil), script.inputs...)
	script.mu.Unlock()
	if len(inputs) != 2 {
		t.Fatalf("model calls = %d, want initial call and one post-exit call", len(inputs))
	}
	for i, input := range inputs {
		var content strings.Builder
		for _, message := range input {
			content.WriteString(message.Content)
			for _, part := range message.MultiContent {
				content.WriteString(part.Text)
			}
		}
		count := strings.Count(content.String(), planGuidanceText)
		want := 1
		if i == 1 {
			want = 0
		}
		if count != want {
			t.Fatalf("model request %d contains Plan guidance %d times, want %d; input: %s", i+1, count, want, content.String())
		}
	}
	state, err := backend.ReadWork(ctx, sessionID)
	if err != nil || state.Plan.Active {
		t.Fatalf("persisted Plan after scripted exit = %+v / %v, want inactive", state.Plan, err)
	}
}
