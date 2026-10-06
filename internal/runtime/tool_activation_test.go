package runtime

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/schema"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/tools"
)

func TestActivatedToolVisibilityMiddleware(t *testing.T) {
	lazy := &schema.ToolInfo{Name: "lazy.tool"}
	mw := newActivatedToolVisibilityMiddleware(map[string]*schema.ToolInfo{"lazy.tool": lazy}, []string{"lazy.tool"})
	inner, ok := mw.(*activatedToolVisibilityMiddleware)
	if !ok {
		t.Fatalf("middleware type = %T", mw)
	}
	state := &adk.ChatModelAgentState{}
	if _, _, err := inner.BeforeModelRewriteState(context.Background(), state, nil); err != nil {
		t.Fatal(err)
	}
	if len(state.ToolInfos) != 0 {
		t.Fatalf("unbound ctx injected infos: %v", state.ToolInfos)
	}
	activation := tools.NewToolActivation()
	ctx := tools.WithToolActivation(context.Background(), activation)
	if _, _, err := inner.BeforeModelRewriteState(ctx, state, nil); err != nil {
		t.Fatal(err)
	}
	if len(state.ToolInfos) != 0 {
		t.Fatalf("unactivated name injected infos: %v", state.ToolInfos)
	}
	activation.Activate("lazy.tool")
	if _, _, err := inner.BeforeModelRewriteState(ctx, state, nil); err != nil {
		t.Fatal(err)
	}
	if len(state.ToolInfos) != 1 || state.ToolInfos[0].Name != "lazy.tool" {
		t.Fatalf("activated infos = %v", state.ToolInfos)
	}
	// Deactivation strips it from a later generation's projection.
	activation.Deactivate("lazy.tool")
	state = &adk.ChatModelAgentState{}
	if _, _, err := inner.BeforeModelRewriteState(ctx, state, nil); err != nil {
		t.Fatal(err)
	}
	if len(state.ToolInfos) != 0 {
		t.Fatalf("deactivated infos = %v", state.ToolInfos)
	}
}

func TestSuppressedToolVisibilityMiddleware(t *testing.T) {
	mw := newSuppressedToolVisibilityMiddleware(map[string]struct{}{"secret.tool": {}})
	inner := mw.(*suppressedToolVisibilityMiddleware)
	state := &adk.ChatModelAgentState{ToolInfos: []*schema.ToolInfo{
		{Name: "open.tool"}, {Name: "secret.tool"}, nil,
	}}
	if _, _, err := inner.BeforeModelRewriteState(context.Background(), state, nil); err != nil {
		t.Fatal(err)
	}
	if len(state.ToolInfos) != 1 || state.ToolInfos[0].Name != "open.tool" {
		t.Fatalf("suppressed infos = %v", state.ToolInfos)
	}
}

func TestSessionToolActivationJournalFold(t *testing.T) {
	f := newShellFixture(t, domain.ApprovalPolicyAuto)
	ctx := context.Background()
	runID, err := f.service.RunShell(ctx, f.sessionID, "echo activation-fold")
	if err != nil {
		t.Fatal(err)
	}
	waitForRunStatus(t, f.backend, runID, domain.RunCompleted)

	svc := f.service
	if err := svc.SetToolActivation(ctx, f.sessionID, []string{"lazy.tool"}, true); err != nil {
		t.Fatalf("activate: %v", err)
	}
	if got := svc.ToolActivation(ctx, f.sessionID); len(got) != 1 || got[0] != "lazy.tool" {
		t.Fatalf("activation snapshot = %v", got)
	}
	// Drop the in-memory tracker; the journal fold must reproduce the state.
	svc.dropSessionToolActivation(f.sessionID)
	activation := svc.sessionToolActivation(ctx, f.sessionID)
	if !activation.Has("lazy.tool") {
		t.Fatal("journal fold lost the activation")
	}

	// A finished tool_search in the journal activates its matches too.
	result, _ := json.Marshal(map[string]any{"matches": []string{"found.a", "found.b"}})
	if _, err := svc.recordSyntheticSessionEvent(ctx, f.sessionID, toolExposureRunID(f.sessionID), domain.EventToolFinished, payloadToolFinished{
		ToolCallID: "call-1", ToolName: officialToolSearchName, Result: string(result),
	}); err != nil {
		t.Fatalf("journal tool.finished: %v", err)
	}
	svc.dropSessionToolActivation(f.sessionID)
	activation = svc.sessionToolActivation(ctx, f.sessionID)
	if !activation.Has("lazy.tool") || !activation.Has("found.a") || !activation.Has("found.b") {
		t.Fatalf("folded activation = %v", activation.Snapshot())
	}

	// Deactivate journals a fold that removes the name.
	if err := svc.SetToolActivation(ctx, f.sessionID, []string{"lazy.tool"}, false); err != nil {
		t.Fatalf("deactivate: %v", err)
	}
	svc.dropSessionToolActivation(f.sessionID)
	activation = svc.sessionToolActivation(ctx, f.sessionID)
	if activation.Has("lazy.tool") || !activation.Has("found.a") {
		t.Fatalf("deactivation fold = %v", activation.Snapshot())
	}
}
