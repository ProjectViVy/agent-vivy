package stream

import (
	"strings"
	"testing"

	"agent-vivy/sdk/tui/surface"
)

func TestProjectionKeepsReasoningContinuousAcrossEmptyDelta(t *testing.T) {
	p := Projection{}
	nextID := idFactory()
	p.Apply(Notice{Kind: "reasoning", Delta: "这"}, nextID)
	p.Apply(Notice{Kind: "delta"}, nextID)
	p.Apply(Notice{Kind: "reasoning", Delta: "是一句"}, nextID)
	p.Apply(Notice{Kind: "delta", Delta: ""}, nextID)
	p.Apply(Notice{Kind: "reasoning", Delta: "话🙂\n继续"}, nextID)
	if len(p.Messages) != 1 || !p.Messages[0].Reasoning || p.Messages[0].Content != "这是一句话🙂\n继续" {
		t.Fatalf("reasoning messages = %+v", p.Messages)
	}
	p.Apply(Notice{Kind: "delta", Delta: "答案"}, nextID)
	if len(p.Messages) != 2 || p.Messages[0].Streaming || !p.Messages[0].Reasoning || p.Messages[1].Reasoning || p.Messages[1].Content != "答案" {
		t.Fatalf("answer boundary = %+v", p.Messages)
	}
}

func TestProjectionHandlesToolGateAndFailure(t *testing.T) {
	p := Projection{Messages: []surface.Message{{ID: "asst", Role: surface.RoleAssistant, Streaming: true}}}
	nextID := idFactory()
	p.Apply(Notice{Kind: "gate", Gate: &GatePrompt{Kind: "approval", ID: "a1", Title: "write", Body: "README.md"}}, nextID)
	if p.Gate == nil || p.Gate.ID != "a1" || len(p.Messages) != 2 || p.Messages[1].Tool == nil {
		t.Fatalf("gate projection = %+v gate=%+v", p.Messages, p.Gate)
	}
	p.Apply(Notice{Kind: "tool_finished", Message: "write", Line: "ok"}, nextID)
	if p.Messages[1].Tool.Status != "done" || p.Messages[1].Tool.Result != "ok" {
		t.Fatalf("tool projection = %+v", p.Messages)
	}
	if !p.Apply(Notice{Kind: "done", Failed: true, Message: "cancelled"}, nextID) || p.Gate != nil {
		t.Fatalf("terminal projection = %+v gate=%+v", p.Messages, p.Gate)
	}
	if !strings.Contains(p.Messages[len(p.Messages)-1].Content, "cancelled") {
		t.Fatalf("terminal message = %+v", p.Messages)
	}
}

func TestProjectionCarriesApprovalPreviewIntoGateAndToolCard(t *testing.T) {
	p := Projection{Messages: []surface.Message{{ID: "requested", Role: surface.RoleTool, Tool: &surface.ToolCard{ToolName: "write", ToolCallID: "call-1", Status: "pending", Preview: `{"secret":"must-not-survive"}`}}}}
	nextID := idFactory()
	risks := []string{"overwrite"}
	p.Apply(Notice{Kind: "gate", Gate: &GatePrompt{
		Kind: "approval", ID: "a1", ToolCallID: "call-1", Title: "write", Body: "arguments", Action: "write", Target: "a.go",
		PreconditionHash: strings.Repeat("a", 64), Preview: "--- a/a.go\n+++ b/a.go\n@@ -1 +1 @@\n-old\n+new", Risks: risks,
	}}, nextID)
	if p.Gate == nil || p.Gate.Action != "write" || p.Gate.Target != "a.go" || len(p.Gate.PreconditionHash) != 64 || len(p.Gate.Risks) != 1 {
		t.Fatalf("gate = %+v", p.Gate)
	}
	if len(p.Messages) != 1 || p.Messages[0].Tool == nil || !strings.Contains(p.Messages[0].Tool.Preview, "+new") || strings.Contains(p.Messages[0].Tool.Preview, "must-not-survive") {
		t.Fatalf("tool card = %+v", p.Messages)
	}
	risks[0] = "mutated"
	if p.Gate.Risks[0] != "overwrite" {
		t.Fatalf("projection retained caller-owned risk slice: %#v", p.Gate.Risks)
	}
}

func TestProjectionV3CompletionRequiresValidatedMetadata(t *testing.T) {
	nextID := idFactory()
	p := Projection{}
	p.Apply(Notice{Kind: "delta", Delta: "streamed"}, nextID)
	if done := p.Apply(Notice{Kind: "model_completed", HasCompleted: true}, nextID); !done {
		t.Fatal("metadata-less completion was accepted")
	}
	if !strings.Contains(p.ProtocolError, "missing validated metadata") {
		t.Fatalf("protocol error = %q", p.ProtocolError)
	}

	p = Projection{}
	p.Apply(Notice{Kind: "delta", Delta: "streamed answer"}, nextID)
	done := p.Apply(Notice{Kind: "model_completed", PayloadVersion: 3, HasCompleted: true, Completion: &ModelCompletionMetadata{
		ByteLen: 99,
	}}, nextID)
	if !done || p.ProtocolError == "" {
		t.Fatalf("mismatched completion metadata was accepted: done=%v err=%q", done, p.ProtocolError)
	}
}

func TestProjectionV3CompletionClosesDeltaBoundaryWithoutRewriting(t *testing.T) {
	nextID := idFactory()
	p := Projection{}
	p.Apply(Notice{Kind: "delta", PayloadVersion: 3, Delta: "streamed answer"}, nextID)
	p.Apply(Notice{Kind: "model_completed", PayloadVersion: 3, HasCompleted: true, Completion: &ModelCompletionMetadata{
		ByteLen: len([]byte("streamed answer")),
	}}, nextID)

	if len(p.Messages) != 1 || p.Messages[0].Content != "streamed answer" || p.Messages[0].Streaming {
		t.Fatalf("v3 delta boundary projection = %+v", p.Messages)
	}
}

func TestProjectionPairsConcurrentSameNameToolsByCallID(t *testing.T) {
	p := Projection{}
	nextID := idFactory()
	p.Apply(Notice{Kind: "tool_requested", ToolCallID: "call_1", Message: "read_file", Line: "a"}, nextID)
	p.Apply(Notice{Kind: "tool_requested", ToolCallID: "call_2", Message: "read_file", Line: "b"}, nextID)
	p.Apply(Notice{Kind: "tool_finished", ToolCallID: "call_1", Message: "read_file", Line: "result a"}, nextID)
	if p.Messages[0].Tool.Status != "done" || p.Messages[0].Tool.Result != "result a" || p.Messages[1].Tool.Status != "pending" {
		t.Fatalf("tool cards = %+v", p.Messages)
	}
	p.Apply(Notice{Kind: "tool_finished", ToolCallID: "call_2", Message: "read_file", Line: "result b"}, nextID)
	if p.Messages[1].Tool.Status != "done" || p.Messages[1].Tool.Result != "result b" {
		t.Fatalf("second tool card = %+v", p.Messages[1])
	}
}

func TestProjectionDoesNotFallbackWhenToolCallIDIsPresent(t *testing.T) {
	p := Projection{}
	nextID := idFactory()
	p.Apply(Notice{Kind: "tool_requested", ToolCallID: "call_1", Message: "read_file"}, nextID)
	p.Apply(Notice{Kind: "tool_finished", ToolCallID: "wrong", Message: "read_file", Line: "wrong result"}, nextID)
	if p.Messages[0].Tool.Status != "pending" || p.Messages[0].Tool.Result != "" {
		t.Fatalf("mismatched non-empty id fell back by name: %+v", p.Messages[0].Tool)
	}
}

func idFactory() func(string) string {
	var i int
	return func(prefix string) string {
		i++
		return prefix + "_" + string(rune('0'+i))
	}
}
