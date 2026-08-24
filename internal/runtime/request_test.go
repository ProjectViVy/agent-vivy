package runtime

import (
	"testing"

	"github.com/cloudwego/eino/schema"
)

func TestDigestModelRequestFingerprintsPreambleAndTools(t *testing.T) {
	msgs := []*schema.Message{
		schema.SystemMessage("preamble"),
		schema.UserMessage("hi"),
		schema.AssistantMessage("", []schema.ToolCall{{
			ID: "c1", Function: schema.FunctionCall{Name: "echo_info", Arguments: `{"text":"x"}`},
		}}),
		schema.ToolMessage("x", "c1"),
	}
	got := digestModelRequest(msgs, []string{"echo_info"})
	if got.PreambleSHA256 != sha256Hex([]byte("preamble")) || got.PreambleBytes != len("preamble") {
		t.Fatalf("preamble digest = %+v", got)
	}
	if len(got.SelectedTools) != 1 || got.SelectedTools[0] != "echo_info" {
		t.Fatalf("selected tools = %v", got.SelectedTools)
	}
	if got.Messages[2].Role != "assistant" || got.Messages[2].ToolName != "echo_info" || got.Messages[2].ToolCallID != "c1" {
		t.Fatalf("assistant row = %+v", got.Messages[2])
	}
	if got.Messages[3].Role != "tool" || got.Messages[3].ToolCallID != "c1" {
		t.Fatalf("tool row = %+v", got.Messages[3])
	}
}
