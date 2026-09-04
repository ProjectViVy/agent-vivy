package runtime

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

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

func TestMapperPublishesStreamChunkBeforeEOF(t *testing.T) {
	reader, writer := schema.Pipe[*schema.Message](1)
	m := newEventMapper("run-live", 4096)
	emitted := make(chan []domain.RunEvent, 2)
	done := make(chan error, 1)
	go func() {
		done <- m.onStreamEventEach(&adk.TypedMessageVariant[*schema.Message]{
			IsStreaming: true, MessageStream: reader, Role: schema.Assistant,
		}, func(events []domain.RunEvent) error {
			emitted <- append([]domain.RunEvent(nil), events...)
			return nil
		})
	}()
	writer.Send(&schema.Message{Role: schema.Assistant, ReasoningContent: "这"}, nil)
	select {
	case events := <-emitted:
		if len(events) != 1 || events[0].Type != domain.EventModelReasoningDelta {
			t.Fatalf("first live batch = %+v", events)
		}
	case <-time.After(time.Second):
		t.Fatal("first stream chunk was retained until EOF")
	}
	writer.Close()
	if err := <-done; err != nil {
		t.Fatalf("stream mapping: %v", err)
	}
}

func TestMapperSplitsOversizedDeltasWithoutLosingText(t *testing.T) {
	m := newEventMapper("run-split", 64)
	want := strings.Repeat("中文🙂", 20)
	events := m.deltaEvents(want)
	if len(events) < 2 {
		t.Fatalf("oversized delta produced %d event(s)", len(events))
	}
	var got strings.Builder
	for _, event := range events {
		if len(event.Payload) > m.maxPayload {
			t.Fatalf("payload bytes = %d, max %d", len(event.Payload), m.maxPayload)
		}
		var payload payloadModelDelta
		if err := json.Unmarshal(event.Payload, &payload); err != nil {
			t.Fatal(err)
		}
		got.WriteString(payload.Delta)
	}
	if got.String() != want {
		t.Fatalf("reassembled delta differs: got %q want %q", got.String(), want)
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

func TestMapperReasoningOnlyChunksDoNotEmitEmptyDeltas(t *testing.T) {
	reader, writer := schema.Pipe[*schema.Message](2)
	m := newEventMapper("run-reasoning", 4096)
	go func() {
		writer.Send(&schema.Message{Role: schema.Assistant, ReasoningContent: "这"}, nil)
		writer.Send(&schema.Message{Role: schema.Assistant, ReasoningContent: "是一句话"}, nil)
		writer.Close()
	}()
	events, err := m.onStreamEvent(&adk.TypedMessageVariant[*schema.Message]{
		IsStreaming: true, MessageStream: reader, Role: schema.Assistant,
	})
	if err != nil {
		t.Fatalf("map reasoning stream: %v", err)
	}
	if len(events) != 2 {
		t.Fatalf("events = %+v, want exactly two reasoning deltas", events)
	}
	for _, event := range events {
		if event.Type != domain.EventModelReasoningDelta {
			t.Fatalf("event type = %s, want only reasoning deltas", event.Type)
		}
	}
	if flushed := m.onTurnEnd(); len(flushed) != 0 {
		t.Fatalf("reasoning-only stream flushed assistant content: %+v", flushed)
	}
}

func TestMapperStreamingToolCallFlushesPreambleAndFencesNextRound(t *testing.T) {
	reader, writer := schema.Pipe[*schema.Message](2)
	m := newEventMapper("run-stream-tool", 4096)
	go func() {
		writer.Send(&schema.Message{Role: schema.Assistant, Content: "before tool"}, nil)
		writer.Send(&schema.Message{Role: schema.Assistant, ToolCalls: []schema.ToolCall{{
			ID: "call-1", Function: schema.FunctionCall{Name: "echo_info", Arguments: `{}`},
		}}}, nil)
		writer.Close()
	}()
	events, err := m.onStreamEvent(&adk.TypedMessageVariant[*schema.Message]{
		IsStreaming: true, MessageStream: reader, Role: schema.Assistant,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 2 || events[0].Type != domain.EventModelDelta || events[1].Type != domain.EventToolRequested {
		t.Fatalf("streaming tool boundary events = %+v", events)
	}
	if !strings.Contains(string(events[0].Payload), `"delta":"before tool"`) || m.hasPending || m.pendingText.Len() != 0 {
		t.Fatalf("streaming preamble was not flushed: event=%s pending=%q", events[0].Payload, m.pendingText.String())
	}
	next, err := m.onMessageEvent(&adk.TypedMessageVariant[*schema.Message]{Message: &schema.Message{Role: schema.Assistant, Content: "after tool"}})
	if err != nil || len(next) != 1 || !strings.Contains(string(next[0].Payload), `"content":"after tool"`) || strings.Contains(string(next[0].Payload), "before tool") {
		t.Fatalf("next round was contaminated: events=%+v err=%v", next, err)
	}
}

func TestMapperMaterializedObservedToolCallDoesNotDuplicatePreamble(t *testing.T) {
	m := newEventMapper("run-observed-tool", 4096)
	m.beginObservedStream()
	observed := m.observeStreamChunk(&schema.Message{Role: schema.Assistant, Content: "before tool"})
	materialized, err := m.onMessageEvent(&adk.TypedMessageVariant[*schema.Message]{Message: &schema.Message{
		Role:      schema.Assistant,
		ToolCalls: []schema.ToolCall{{ID: "call-1", Function: schema.FunctionCall{Name: "echo_info", Arguments: `{}`}}},
	}})
	if err != nil {
		t.Fatal(err)
	}
	events := append(observed, materialized...)
	if len(events) != 2 || events[0].Type != domain.EventModelDelta || events[1].Type != domain.EventToolRequested {
		t.Fatalf("observed materialization duplicated preamble: %+v", events)
	}
	if m.hasPending || m.pendingText.Len() != 0 {
		t.Fatalf("materialized tool call retained pending text %q", m.pendingText.String())
	}
}
