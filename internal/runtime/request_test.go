package runtime

import (
	"testing"

	"github.com/cloudwego/eino/schema"
)

func TestDigestModelRequestFingerprintsPreambleAndTools(t *testing.T) {
	msgs := []*schema.AgenticMessage{
		testSystem("preamble"),
		testUser("hi"),
		testToolCalls(testCall("c1", "echo_info", `{"text":"x"}`)),
		testToolResult("c1", "", "x"),
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

func TestDigestModelRequestIncludesTextMultiContent(t *testing.T) {
	msg := agenticUserBlocks([]*schema.ContentBlock{
		agenticTextBlock("inspect"),
		agenticTextBlock("\n\n[project file: main.go]\npackage main"),
	})
	want := []byte("inspect\n\n[project file: main.go]\npackage main")
	got := digestModelRequest([]*schema.AgenticMessage{msg}, nil)
	if len(got.Messages) != 1 || got.Messages[0].ByteLen != len(want) || got.Messages[0].ContentSHA256 != sha256Hex(want) {
		t.Fatalf("multicontent digest = %+v", got.Messages)
	}
}
