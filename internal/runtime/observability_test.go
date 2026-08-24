package runtime

import (
	"testing"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/schema"

	"agent-vivy/internal/domain"
)

func TestMapperEmitsReasoningAndUsageEvents(t *testing.T) {
	m := newEventMapper("run-observe", 4096)
	events, err := m.onMessageEvent(&adk.TypedMessageVariant[*schema.Message]{
		Role: schema.Assistant,
		Message: &schema.Message{
			Role:             schema.Assistant,
			Content:          "final answer",
			ReasoningContent: "short reasoning summary",
			ResponseMeta: &schema.ResponseMeta{Usage: &schema.TokenUsage{
				PromptTokens: 11, CompletionTokens: 7, TotalTokens: 18,
				CompletionTokensDetails: schema.CompletionTokensDetails{ReasoningTokens: 3},
			}},
		},
	})
	if err != nil {
		t.Fatalf("map message: %v", err)
	}
	if len(events) != 3 || events[0].Type != domain.EventModelReasoningDelta ||
		events[1].Type != domain.EventModelUsage || events[2].Type != domain.EventModelCompleted {
		t.Fatalf("events = %+v, want reasoning/usage/completed", events)
	}
}

func TestMapperEmitsProviderRetryWithoutErrorLeak(t *testing.T) {
	m := newEventMapper("run-retry", 4096)
	events, err := m.onEvent(&adk.AgentEvent{Err: &adk.WillRetryError{
		ErrStr: "provider secret and URL must not be durable", RetryAttempt: 2,
	}})
	if err != nil {
		t.Fatalf("retry event: %v", err)
	}
	if len(events) != 1 || events[0].Type != domain.EventProviderRetry {
		t.Fatalf("events = %+v, want one provider.retry", events)
	}
	if string(events[0].Payload) == "" || string(events[0].Payload) == "provider secret and URL must not be durable" {
		t.Fatalf("retry payload leaked provider error: %s", events[0].Payload)
	}
}

func TestMapperEmitsStallForSlowProviderStream(t *testing.T) {
	reader, writer := schema.Pipe[*schema.Message](1)
	m := newEventMapper("run-stall", 4096)
	m.stallThreshold = 0
	go func() {
		writer.Send(&schema.Message{Role: schema.Assistant, Content: "chunk"}, nil)
		writer.Close()
	}()
	events, err := m.onStreamEvent(&adk.TypedMessageVariant[*schema.Message]{
		IsStreaming: true, MessageStream: reader, Role: schema.Assistant,
	})
	if err != nil {
		t.Fatalf("map stream: %v", err)
	}
	seen := map[domain.EventType]bool{}
	for _, ev := range events {
		seen[ev.Type] = true
	}
	if !seen[domain.EventModelDelta] || !seen[domain.EventProviderStall] {
		t.Fatalf("events = %+v, want model.delta and provider.stall", events)
	}
}
