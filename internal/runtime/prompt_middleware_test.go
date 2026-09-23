package runtime

import (
	"context"
	"strings"
	"testing"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/schema"
)

func TestLiteralGenModelInputKeepsMaskBraces(t *testing.T) {
	ctx := context.Background()
	instruction := `Selected mask data: {"body":"{system} {{persona}}"}`
	input := &adk.AgentInput{Messages: []*schema.Message{schema.UserMessage("hello")}}

	msgs, err := literalGenModelInput(ctx, instruction, input)
	if err != nil {
		t.Fatal(err)
	}
	if len(msgs) != 2 || msgs[0].Role != "system" || msgs[0].Content != instruction {
		t.Fatalf("literal instruction was changed: %+v", msgs)
	}
	if msgs[1].Content != "hello" {
		t.Fatalf("input message was not preserved: %+v", msgs[1])
	}
}

func TestPromptMiddlewareDoesNotDuplicateAuthoritativeInstruction(t *testing.T) {
	snapshot, err := buildPromptSnapshot(PromptInput{
		RunID: "run-middleware", GenerationID: "generation-middleware", Capture: promptCapture(nil), Face: "web",
	})
	if err != nil {
		t.Fatal(err)
	}
	middleware := newPromptMiddleware(0)
	ctx := withRunPrompt(context.Background(), snapshot)
	_, first, err := middleware.BeforeAgent(ctx, &adk.ChatModelAgentContext{Instruction: composeStaticInstruction()})
	if err != nil {
		t.Fatal(err)
	}
	_, second, err := middleware.BeforeAgent(ctx, &adk.ChatModelAgentContext{Instruction: first.Instruction + "\ntransient tool instruction"})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(second.Instruction, "You are Vivy") != 1 {
		t.Fatalf("authoritative persona duplicated: %q", second.Instruction)
	}
	if !strings.HasSuffix(second.Instruction, "transient tool instruction") {
		t.Fatalf("transient instruction was dropped: %q", second.Instruction)
	}
}

func TestPromptInstructionReservationIncludesLegacyStaticInstruction(t *testing.T) {
	got, err := promptInstructionReservation(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	want := projectedContextBytes([]*schema.Message{schema.SystemMessage(composeStaticInstruction())})
	if got != want {
		t.Fatalf("legacy prompt reservation = %d, want %d", got, want)
	}
}
