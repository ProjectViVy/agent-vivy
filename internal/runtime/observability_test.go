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
	if len(events) != 4 || events[0].Type != domain.EventModelReasoningDelta ||
		events[1].Type != domain.EventModelUsage || events[2].Type != domain.EventModelDelta ||
		events[3].Type != domain.EventModelCompleted {
		t.Fatalf("events = %+v, want reasoning/usage/delta/completed", events)
	}
	if got := payloadDeltaOf(t, events[2].Payload); got != "final answer" {
		t.Fatalf("assistant delta = %q, want final answer", got)
	}
	completed := payloadCompletedOf(t, events[3].Payload)
	if completed.ContentSHA256 != sha256Hex([]byte("final answer")) || completed.ByteLen != len([]byte("final answer")) {
		t.Fatalf("assistant completion = %+v", completed)
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

// A provider may deliver a complete assistant message through the non-stream
// mapper path. It still has to use bounded v1 deltas plus the v2 completion
// digest, never put the whole response in model.completed.
func TestMapperBoundsLargeNonStreamingAssistantPayload(t *testing.T) {
	const budget = 256
	content := strings.Repeat("large non-stream answer ", 5000)
	m := newEventMapper("run-large-non-stream", budget)
	events, err := m.onMessageEvent(&adk.TypedMessageVariant[*schema.Message]{
		Message: &schema.Message{Role: schema.Assistant, Content: content},
	})
	if err != nil {
		t.Fatalf("map non-stream message: %v", err)
	}
	if len(events) < 2 || events[len(events)-1].Type != domain.EventModelCompleted {
		t.Fatalf("events = %d/%+v, want bounded deltas followed by completion", len(events), events)
	}
	var got strings.Builder
	for i, event := range events[:len(events)-1] {
		if event.Type != domain.EventModelDelta {
			t.Fatalf("event %d type = %s, want model.delta", i, event.Type)
		}
		if event.PayloadVersion != 1 || len(event.Payload) > budget {
			t.Fatalf("delta event %d version/bytes = %d/%d, want v1 <= %d", i, event.PayloadVersion, len(event.Payload), budget)
		}
		got.WriteString(payloadDeltaOf(t, event.Payload))
	}
	if got.String() != content {
		t.Fatalf("reassembled non-stream content length = %d, want %d", len(got.String()), len(content))
	}
	completedEvent := events[len(events)-1]
	if completedEvent.PayloadVersion != 2 || len(completedEvent.Payload) > budget {
		t.Fatalf("completion version/bytes = %d/%d, want v2 <= %d", completedEvent.PayloadVersion, len(completedEvent.Payload), budget)
	}
	completed := payloadCompletedOf(t, completedEvent.Payload)
	if completed.ContentSHA256 != sha256Hex([]byte(content)) || completed.ByteLen != len([]byte(content)) {
		t.Fatalf("large non-stream completion = %+v", completed)
	}
}

func TestMapperV2CompletionFitsMinimumConfiguredPayloadBudget(t *testing.T) {
	event := newEventMapper("run-min-completion", 128).completedEvent(strings.Repeat("界", 1000))
	if event.PayloadVersion != 2 || len(event.Payload) > 128 {
		t.Fatalf("completion version/bytes = %d/%d, want v2 <= 128", event.PayloadVersion, len(event.Payload))
	}
}

// Text attached to a non-streaming assistant tool-call message is a model
// preamble. It must be emitted once as a delta before tool.requested, while
// the following model round starts with a fresh accumulator.
func TestMapperNonStreamingToolCallFlushesPreamble(t *testing.T) {
	m := newEventMapper("run-non-stream-tool-preamble", 4096)
	events, err := m.onMessageEvent(&adk.TypedMessageVariant[*schema.Message]{
		Message: &schema.Message{
			Role: schema.Assistant, Content: "before tool",
			ToolCalls: []schema.ToolCall{{ID: "call-1", Function: schema.FunctionCall{Name: "echo_info", Arguments: `{}`}}},
		},
	})
	if err != nil {
		t.Fatalf("map non-stream tool call: %v", err)
	}
	if len(events) != 2 || events[0].Type != domain.EventModelDelta || events[1].Type != domain.EventToolRequested {
		t.Fatalf("tool preamble events = %+v, want delta/tool.requested", events)
	}
	if got := payloadDeltaOf(t, events[0].Payload); got != "before tool" {
		t.Fatalf("tool preamble delta = %q", got)
	}
	if m.hasPending || m.pendingText.Len() != 0 {
		t.Fatalf("tool preamble remained pending: %q", m.pendingText.String())
	}

	next, err := m.onMessageEvent(&adk.TypedMessageVariant[*schema.Message]{
		Message: &schema.Message{Role: schema.Assistant, Content: "after tool"},
	})
	if err != nil {
		t.Fatalf("map post-tool message: %v", err)
	}
	if len(next) != 2 || next[0].Type != domain.EventModelDelta || next[1].Type != domain.EventModelCompleted {
		t.Fatalf("post-tool events = %+v, want delta/completed", next)
	}
	if got := payloadDeltaOf(t, next[0].Payload); got != "after tool" {
		t.Fatalf("post-tool delta = %q", got)
	}
	completed := payloadCompletedOf(t, next[1].Payload)
	if completed.ContentSHA256 != sha256Hex([]byte("after tool")) || completed.ByteLen != len([]byte("after tool")) {
		t.Fatalf("post-tool completion = %+v", completed)
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
	if err != nil || len(next) != 2 || next[0].Type != domain.EventModelDelta || next[1].Type != domain.EventModelCompleted || strings.Contains(string(next[1].Payload), "before tool") {
		t.Fatalf("next round was contaminated: events=%+v err=%v", next, err)
	}
	if got := payloadDeltaOf(t, next[0].Payload); got != "after tool" {
		t.Fatalf("next-round delta = %q, want after tool", got)
	}
	completed := payloadCompletedOf(t, next[1].Payload)
	if completed.ContentSHA256 != sha256Hex([]byte("after tool")) || completed.ByteLen != len([]byte("after tool")) {
		t.Fatalf("next-round completion = %+v", completed)
	}
}

func TestMapperConcatenatesStreamingToolCallIdentityAndArguments(t *testing.T) {
	reader, writer := schema.Pipe[*schema.Message](3)
	m := newEventMapper("run-stream-tool-fragments", 4096)
	index := 0
	go func() {
		writer.Send(&schema.Message{Role: schema.Assistant, ToolCalls: []schema.ToolCall{{
			Index: &index, ID: "call-1", Type: "function",
			Function: schema.FunctionCall{Name: "list_dir", Arguments: `{"path":"`},
		}}}, nil)
		writer.Send(&schema.Message{ToolCalls: []schema.ToolCall{{
			Index: &index, Function: schema.FunctionCall{Arguments: "."},
		}}}, nil)
		writer.Send(&schema.Message{ToolCalls: []schema.ToolCall{{
			Index: &index, Function: schema.FunctionCall{Arguments: `"}`},
		}}}, nil)
		writer.Close()
	}()
	events, err := m.onStreamEvent(&adk.TypedMessageVariant[*schema.Message]{
		IsStreaming: true, MessageStream: reader, Role: schema.Assistant,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 || events[0].Type != domain.EventToolRequested {
		t.Fatalf("events = %+v, want one tool.requested", events)
	}
	var requested payloadToolRequested
	if err := json.Unmarshal(events[0].Payload, &requested); err != nil {
		t.Fatalf("decode tool.requested: %v", err)
	}
	if requested.ToolCallID != "call-1" || requested.ToolName != "list_dir" || requested.Args["path"] != "." {
		t.Fatalf("tool.requested = %+v", requested)
	}

	result, err := m.toolResultEventsParts("", "", "done", nil, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(result) != 2 || result[1].Type != domain.EventToolFinished {
		t.Fatalf("result events = %+v", result)
	}
	var finished payloadToolFinished
	if err := json.Unmarshal(result[1].Payload, &finished); err != nil {
		t.Fatalf("decode tool.finished: %v", err)
	}
	if finished.ToolCallID != requested.ToolCallID || finished.ToolName != requested.ToolName {
		t.Fatalf("tool identity requested=%+v finished=%+v", requested, finished)
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
