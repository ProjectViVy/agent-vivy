package runtime

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/cloudwego/eino/schema"

	"agent-vivy/internal/domain"
)

// TestValidateModelInputAcceptsLegitimateTurns guards against a refusal that
// would break a feed the provider can accept: multiple calls, multiple results
// in one message, and several tool rounds.
func TestValidateModelInputAcceptsLegitimateTurns(t *testing.T) {
	valid := []struct {
		name string
		msgs []*schema.AgenticMessage
	}{
		{
			name: "plain conversation",
			msgs: []*schema.AgenticMessage{testSystem("sys"), testUser("hi"), testAssistant("hello"), testUser("again")},
		},
		{
			name: "assistant reasoning and text",
			msgs: []*schema.AgenticMessage{
				testUser("hi"),
				{
					Role: schema.AgenticRoleTypeAssistant,
					ContentBlocks: []*schema.ContentBlock{
						schema.NewContentBlock(&schema.Reasoning{Text: "consider"}),
						schema.NewContentBlock(&schema.AssistantGenText{Text: "answer"}),
					},
				},
			},
		},
		{
			name: "two calls answered in one result message",
			msgs: []*schema.AgenticMessage{
				testUser("go"),
				testToolCalls(testCall("a", "echo_info", `{}`), testCall("b", "read_file", `{}`)),
				agenticUserBlocks(append(
					testToolResult("a", "echo_info", "a").ContentBlocks,
					testToolResult("b", "read_file", "b").ContentBlocks...,
				)),
			},
		},
		{
			name: "two tool rounds",
			msgs: []*schema.AgenticMessage{
				testUser("go"),
				testToolCalls(testCall("a", "echo_info", `{}`)),
				testToolResult("a", "echo_info", "a"),
				testToolCalls(testCall("b", "echo_info", `{}`)),
				testToolResult("b", "echo_info", "b"),
				testAssistant("done"),
			},
		},
		{
			name: "same call id reused on a later turn",
			msgs: []*schema.AgenticMessage{
				testUser("go"),
				testToolCalls(testCall("a", "echo_info", `{}`)),
				testToolResult("a", "echo_info", "first"),
				testToolCalls(testCall("a", "echo_info", `{}`)),
				testToolResult("a", "echo_info", "second"),
			},
		},
	}
	for _, tc := range valid {
		t.Run(tc.name, func(t *testing.T) {
			if err := validateModelInput(tc.msgs); err != nil {
				t.Fatalf("legitimate input refused: %v", err)
			}
		})
	}
}

// TestValidateModelInputRejectsUnsendableShapes covers each rule with the
// smallest input that triggers it.
func TestValidateModelInputRejectsUnsendableShapes(t *testing.T) {
	assistantWith := func(blocks ...*schema.ContentBlock) *schema.AgenticMessage {
		return &schema.AgenticMessage{Role: schema.AgenticRoleTypeAssistant, ContentBlocks: blocks}
	}
	userWith := func(blocks ...*schema.ContentBlock) *schema.AgenticMessage {
		return &schema.AgenticMessage{Role: schema.AgenticRoleTypeUser, ContentBlocks: blocks}
	}
	resultBlock := func(callID string) *schema.ContentBlock {
		return testToolResult(callID, "echo_info", "ok").ContentBlocks[0]
	}

	invalid := []struct {
		name string
		msgs []*schema.AgenticMessage
		want string
	}{
		{
			name: "nil message",
			msgs: []*schema.AgenticMessage{nil},
			want: "is nil",
		},
		{
			name: "system carries a tool call",
			msgs: []*schema.AgenticMessage{{
				Role:          schema.AgenticRoleTypeSystem,
				ContentBlocks: []*schema.ContentBlock{schema.NewContentBlock(testCall("a", "echo_info", `{}`))},
			}},
			want: "system message carries tool blocks",
		},
		{
			name: "assistant carries a tool result",
			msgs: []*schema.AgenticMessage{
				testToolCalls(testCall("a", "echo_info", `{}`)),
				assistantWith(resultBlock("a")),
			},
			want: "assistant message carries a tool result",
		},
		{
			name: "empty assistant message",
			msgs: []*schema.AgenticMessage{testUser("hi"), assistantWith()},
			want: "empty assistant message",
		},
		{
			name: "reasoning-only assistant has no wire content",
			msgs: []*schema.AgenticMessage{testUser("hi"), testReasoning("thinking")},
			want: "empty assistant message",
		},
		{
			name: "tool call without id",
			msgs: []*schema.AgenticMessage{assistantWith(schema.NewContentBlock(testCall("", "echo_info", `{}`)))},
			want: "tool call without a call id",
		},
		{
			name: "tool call without name",
			msgs: []*schema.AgenticMessage{assistantWith(schema.NewContentBlock(testCall("a", "", `{}`)))},
			want: "tool call without a tool name",
		},
		{
			name: "call id reused while still pending",
			msgs: []*schema.AgenticMessage{
				testUser("go"),
				testToolCalls(testCall("a", "echo_info", `{}`)),
				testToolCalls(testCall("a", "echo_info", `{}`)),
			},
			want: "still pending from an earlier call",
		},
		{
			name: "tool result without id",
			msgs: []*schema.AgenticMessage{
				testToolCalls(testCall("a", "echo_info", `{}`)),
				userWith(resultBlock("")),
			},
			want: "tool result without a call id",
		},
		{
			name: "result before its call",
			msgs: []*schema.AgenticMessage{
				userWith(resultBlock("late")),
				testToolCalls(testCall("late", "echo_info", `{}`)),
			},
			want: "call that does not precede it",
		},
		{
			name: "same result twice",
			msgs: []*schema.AgenticMessage{
				testToolCalls(testCall("a", "echo_info", `{}`)),
				userWith(resultBlock("a")),
				userWith(resultBlock("a")),
			},
			want: "answers the same call twice",
		},
		{
			name: "result separated by a user message",
			msgs: []*schema.AgenticMessage{
				testToolCalls(testCall("a", "echo_info", `{}`)),
				testUser("interrupt"),
				userWith(resultBlock("a")),
			},
			want: "separated from its call",
		},
		{
			name: "result mixed with user text",
			msgs: []*schema.AgenticMessage{
				testToolCalls(testCall("a", "echo_info", `{}`)),
				userWith(resultBlock("a"), agenticTextBlock("and a question")),
			},
			want: "mixed with non-result user blocks",
		},
		{
			name: "empty user message",
			msgs: []*schema.AgenticMessage{userWith()},
			want: "empty user message",
		},
		{
			name: "unsupported role",
			msgs: []*schema.AgenticMessage{{Role: schema.AgenticRoleType("tool")}},
			want: "unsupported role",
		},
	}
	for _, tc := range invalid {
		t.Run(tc.name, func(t *testing.T) {
			err := validateModelInput(tc.msgs)
			if err == nil {
				t.Fatal("invalid input accepted")
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("violation = %q, want %q", err, tc.want)
			}
		})
	}
}

// TestModelRequestFingerprintIsStructural: the replay proof must ignore text
// content and change with every structural difference.
func TestModelRequestFingerprintIsStructural(t *testing.T) {
	base := []*schema.AgenticMessage{
		testSystem("sys"),
		testUser("first"),
		testToolCalls(testCall("a", "echo_info", `{"text":"x"}`)),
		testToolResult("a", "echo_info", "x"),
	}
	sameShape := []*schema.AgenticMessage{
		testSystem("a much longer system prompt"),
		testUser("second"),
		testToolCalls(testCall("a", "echo_info", `{"text":"different"}`)),
		testToolResult("a", "echo_info", "different"),
	}
	if modelRequestFingerprint(base) != modelRequestFingerprint(sameShape) {
		t.Fatal("fingerprint changed with text content only")
	}

	reordered := []*schema.AgenticMessage{base[0], base[1], base[3], base[2]}
	if modelRequestFingerprint(base) == modelRequestFingerprint(reordered) {
		t.Fatal("fingerprint ignored block order")
	}
	renamed := []*schema.AgenticMessage{
		base[0], base[1],
		testToolCalls(testCall("b", "echo_info", `{"text":"x"}`)),
		testToolResult("b", "echo_info", "x"),
	}
	if modelRequestFingerprint(base) == modelRequestFingerprint(renamed) {
		t.Fatal("fingerprint ignored call identity")
	}
	if !strings.Contains(modelRequestFingerprint(base), "{result:a}") {
		t.Fatalf("fingerprint = %q", modelRequestFingerprint(base))
	}
}

func TestHasToolTurns(t *testing.T) {
	if hasToolTurns([]*schema.AgenticMessage{testUser("hi"), testAssistant("there")}) {
		t.Fatal("plain conversation reported a tool turn")
	}
	if hasToolTurns([]*schema.AgenticMessage{testToolCalls(testCall("a", "echo_info", `{}`))}) {
		t.Fatal("unanswered call reported a tool result")
	}
	if !hasToolTurns([]*schema.AgenticMessage{testToolResult("a", "echo_info", "ok")}) {
		t.Fatal("tool result not detected")
	}
}

// TestBeginRefusesInvalidInputWithoutCharging answers the three questions a
// refusal must leave in the journal: a request row, its paired finished row,
// and no model-call charge or provider invocation.
func TestBeginRefusesInvalidInputWithoutCharging(t *testing.T) {
	chat := &scriptedUsageModel{chunks: [][]*schema.AgenticMessage{{testAssistant("unused")}}}
	svc, backend, _ := newObservedService(t, chat, DefaultBudgetPolicy())
	ctx := context.Background()
	mustCreateSession(t, backend, "sess-refuse")
	runID := domain.RunID("run_refuse_begin")
	if err := backend.CreateRun(ctx, domain.Run{
		ID: runID, SessionID: "sess-refuse", Status: domain.RunActive, CreatedAt: 1, RootID: runID,
	}); err != nil {
		t.Fatalf("create run: %v", err)
	}

	ledger, err := NewBudgetLedger(DefaultBudgetPolicy())
	if err != nil {
		t.Fatal(err)
	}
	observer := &runModelCallObserver{
		svc: svc, m: newEventMapper(runID, 64<<10), sessionID: "sess-refuse",
		ledger: ledger, source: "main", provider: "test", model: "test-model",
		calls: map[string]*observedModelCall{},
	}
	// The result answers a call that never existed: a pairing the OpenAI wire
	// cannot express.
	invalid := []*schema.AgenticMessage{
		testUser("hi"),
		testToolResult("missing-call", "echo_info", "orphan"),
	}
	_, beginErr := observer.Begin(ctx, modelCallInput{Mode: "generate", Messages: invalid})
	if !errors.Is(beginErr, errModelCallAdmission) || !errors.Is(beginErr, errModelRequestInvalid) {
		t.Fatalf("Begin error = %v, want admission + request-invalid", beginErr)
	}
	if errors.Is(beginErr, context.Canceled) {
		t.Fatalf("refusal claimed cancellation: %v", beginErr)
	}
	if chat.calls != 0 {
		t.Fatalf("provider called %d times for a refused input", chat.calls)
	}
	if got := ledger.Snapshot().Usage.ModelCalls; got != 0 {
		t.Fatalf("model calls charged = %d, want 0", got)
	}

	events := replayAll(t, backend, runID)
	if len(events) != 2 {
		t.Fatalf("journal rows = %d, want request + finished", len(events))
	}
	if events[0].Type != domain.EventModelRequest || events[0].PayloadVersion != 5 {
		t.Fatalf("first row = %s v%d", events[0].Type, events[0].PayloadVersion)
	}
	if events[1].Type != domain.EventModelCallFinished {
		t.Fatalf("second row = %s", events[1].Type)
	}
	var request payloadModelRequestV5
	mustUnmarshal(t, events[0].Payload, &request)
	var finish payloadModelCallFinished
	mustUnmarshal(t, events[1].Payload, &finish)
	if request.CallID == "" || finish.CallID != request.CallID {
		t.Fatalf("request call %q / finish call %q", request.CallID, finish.CallID)
	}
	if finish.Status != "failed" || finish.ResponseComplete || finish.Error == nil || finish.Error.Name != "ModelRequestInvalid" {
		t.Fatalf("finish = %+v", finish)
	}
	if request.Counts.Messages != 2 || request.Counts.ToolResults != 1 || request.Counts.Blocks != 2 {
		t.Fatalf("refused request shape = %+v", request.Counts)
	}
}
