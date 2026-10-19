package runtime

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/storage"
	"agent-vivy/internal/storage/conformance"
	"agent-vivy/internal/tools"
	einotool "github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/compose"
)

// failingStubTool returns a recoverable §5 failure (an ArgError).
type failingStubTool struct{ calls int }

func (t *failingStubTool) Spec() domain.ToolSpec {
	return domain.ToolSpec{Name: "stub_failing", Description: "recoverable failure", Params: map[string]domain.ToolParam{"command": {Desc: "script", Required: true}}}
}
func (t *failingStubTool) InvokableRun(context.Context, json.RawMessage) (string, error) {
	t.calls++
	return "", &tools.ArgError{Field: "path", Reason: "does not exist"}
}

// A recoverable tool error is a model-visible result in a Recap run, not a
// run-fatal error, and a replay after reopen returns the same result.
func TestIssue58RecoverableToolErrorStaysModelVisible(t *testing.T) {
	ctx := context.Background()
	tool := &failingStubTool{}
	f := newIssue58Wire(t, tool)
	source := conformance.Issue58Source("main-reader", true)
	admission := conformance.Issue58Admission(source, "recoverable-error")
	if _, err := f.b.CommitRunAdmission(ctx, admission); err != nil {
		t.Fatal(err)
	}
	args := `{"command":"synthetic"}`
	call := issue58ToolReply(tool.Spec().Name, args)
	mapper := newEventMapper(admission.Run.ID, 64<<10)
	request := mapper.build(domain.EventModelRequest, map[string]any{"source": "main", "input_provenance": storage.InputProvenance{ContentOrigin: "original"}})
	request.PayloadVersion = 4
	requested := mapper.build(domain.EventToolRequested, payloadToolRequested{ToolCallID: "fixture-call", ToolName: tool.Spec().Name, OriginalArguments: args, Args: map[string]any{"command": "synthetic"}})
	if _, err := f.b.Append(ctx, storage.Commit{RunID: admission.Run.ID, Events: []domain.RunEvent{request, requested}}); err != nil {
		t.Fatal(err)
	}
	invoke := func(svc *Service) string {
		t.Helper()
		state := newNudgeState()
		c := svc.newToolOperationCoordinator(admission.Run.ID, source.SessionID)
		runCtx := withRunID(withSessionID(withPolicyProfile(withSelectedTools(withNudgeState(ctx, state), []string{tool.Spec().Name}), domain.PolicyProfileDefault), source.SessionID), admission.Run.ID)
		runCtx = withApprovalPolicy(runCtx, domain.ApprovalPolicyAuto)
		runCtx = svc.withRecapRunPolicy(runCtx, admission.Run.ID, source.SessionID, "fixture", "fixture")
		runCtx = withToolOperationCoordinator(runCtx, c)
		node, err := compose.NewToolNode(runCtx, &compose.ToolsNodeConfig{Tools: []einotool.BaseTool{newEnhancedToolAdapter(newToolAdapter(tool, 128, nil, nil, nil))}})
		if err != nil {
			t.Fatal(err)
		}
		result, err := node.Invoke(runCtx, call)
		if err != nil {
			t.Fatalf("recoverable tool error killed the run: %v", err)
		}
		if len(result) != 1 {
			t.Fatalf("results = %d", len(result))
		}
		if _, ok := state.Failure("fixture-call"); !ok {
			t.Fatal("typed recoverable failure was lost")
		}
		text := result[0].Content
		for _, part := range result[0].UserInputMultiContent {
			text += part.Text
		}
		return text
	}
	first := invoke(f.svc)
	if !strings.Contains(first, "does not exist") || tool.calls != 1 {
		t.Fatalf("model did not see the diagnostic: %q calls=%d", first, tool.calls)
	}
	f.reopen(t, false)
	if replay := invoke(f.svc); replay != first || tool.calls != 1 {
		t.Fatalf("replay differs or re-executed: %q vs %q calls=%d", replay, first, tool.calls)
	}
}

func (t *failingStubTool) ClassifyInvocation(json.RawMessage) (tools.InvocationClass, []string, error) {
	return tools.InvocationSafe, nil, nil
}
