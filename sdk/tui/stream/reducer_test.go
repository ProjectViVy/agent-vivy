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

func idFactory() func(string) string {
	var i int
	return func(prefix string) string {
		i++
		return prefix + "_" + string(rune('0'+i))
	}
}
