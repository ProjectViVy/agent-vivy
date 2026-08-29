package provider

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/cloudwego/eino/schema"

	"agent-vivy/internal/domain"
)

func runMock(t *testing.T, input []*domain.Message) []*domain.Message {
	t.Helper()
	stream, err := NewMock().Stream(context.Background(), input)
	if err != nil {
		t.Fatalf("Stream: %v", err)
	}
	chunks, err := domain.Collect(stream)
	if err != nil {
		t.Fatalf("Collect: %v", err)
	}
	return chunks
}

// The determinism proof: same input, two independent runs, byte-identical
// chunk sequences (FR-3, NFR deterministic mode).
func TestMockDeterministic(t *testing.T) {
	input := []*domain.Message{
		{Role: domain.RoleUser, Content: "hello vivy"},
	}
	first := runMock(t, input)
	second := runMock(t, input)

	if len(first) == 0 {
		t.Fatal("expected at least one chunk")
	}
	if len(first) != len(second) {
		t.Fatalf("chunk counts differ: %d vs %d", len(first), len(second))
	}
	for i := range first {
		if first[i].Content != second[i].Content || first[i].Role != second[i].Role {
			t.Errorf("chunk %d differs: %+v vs %+v", i, first[i], second[i])
		}
	}
}

func TestMockReplyShape(t *testing.T) {
	input := []*domain.Message{
		{Role: domain.RoleUser, Content: "first question"},
		{Role: domain.RoleAssistant, Content: "stale answer"},
		{Role: domain.RoleUser, Content: "latest question"},
	}
	chunks := runMock(t, input)

	var full strings.Builder
	for _, c := range chunks {
		if c.Role != domain.RoleAssistant {
			t.Errorf("chunk role = %q, want assistant", c.Role)
		}
		full.WriteString(c.Content)
	}
	if got := full.String(); got != "mock reply to: latest question" {
		t.Errorf("reassembled reply = %q", got)
	}
	// Fixed chunk size except the tail: multi-chunk proof.
	if len(chunks) < 2 {
		t.Errorf("chunks = %d, want multiple deltas", len(chunks))
	}
	for _, c := range chunks[:len(chunks)-1] {
		if n := len([]rune(c.Content)); n != mockChunkBytes {
			t.Errorf("non-tail chunk rune length = %d, want %d", n, mockChunkBytes)
		}
	}
}

func TestMockEmptyInput(t *testing.T) {
	chunks := runMock(t, nil)
	var full strings.Builder
	for _, c := range chunks {
		full.WriteString(c.Content)
	}
	if !strings.HasPrefix(full.String(), "mock:") {
		t.Errorf("empty-input reply = %q", full.String())
	}
}

func TestMockCanceledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := NewMock().Stream(ctx, nil); err == nil {
		t.Fatal("want error for canceled context")
	}
}

func TestMockHITLScenarioEmitsToolCallThenCompletes(t *testing.T) {
	ref := mockRef{}
	model, err := ref.Model(context.Background(), ModelSpec{ID: "mock:hitl"})
	if err != nil {
		t.Fatalf("Model: %v", err)
	}
	first, err := model.Generate(context.Background(), []*schema.Message{
		schema.UserMessage("e2e approval"),
	})
	if err != nil {
		t.Fatalf("first Generate: %v", err)
	}
	if len(first.ToolCalls) != 1 || first.ToolCalls[0].Function.Name != "write_note" {
		t.Fatalf("first message = %+v", first)
	}
	var args map[string]any
	if err := json.Unmarshal([]byte(first.ToolCalls[0].Function.Arguments), &args); err != nil {
		t.Fatalf("tool args: %v", err)
	}
	if args["content"] != "e2e approval note" {
		t.Fatalf("tool args = %+v", args)
	}
	second, err := model.Generate(context.Background(), []*schema.Message{
		schema.UserMessage("e2e approval"),
		{Role: schema.Tool, Content: "saved"},
	})
	if err != nil {
		t.Fatalf("second Generate: %v", err)
	}
	if second.Content != "mock scenario completed: hitl" || len(second.ToolCalls) != 0 {
		t.Fatalf("second message = %+v", second)
	}
}

func TestMockHITLScenarioSeparatesQuestion(t *testing.T) {
	ref := mockRef{}
	model, err := ref.Model(context.Background(), ModelSpec{ID: "mock:hitl"})
	if err != nil {
		t.Fatalf("Model: %v", err)
	}
	message, err := model.Generate(context.Background(), []*schema.Message{schema.UserMessage("e2e question")})
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if len(message.ToolCalls) != 1 || message.ToolCalls[0].Function.Name != "ask_user" {
		t.Fatalf("message = %+v", message)
	}
}
