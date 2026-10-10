package runtime

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"

	"agent-vivy/internal/domain"
	"github.com/cloudwego/eino/schema"
)

func TestFaithfulMapperPreservesJSONNumbersAndCanonicalRepeats(t *testing.T) {
	m := newEventMapper("run-number-fixture", 4096)
	for i, args := range []string{
		`{"token_count":9007199254740993,"text":"alice@example.com password=synthetic [REDACTED]"}`,
		`{"text":"alice@example.com password=synthetic [REDACTED]","token_count":9007199254740993}`,
	} {
		events := m.toolCallEvents(schema.AssistantMessage("", []schema.ToolCall{{ID: fmt.Sprintf("number-%d", i), Function: schema.FunctionCall{Name: "echo_fixture", Arguments: args}}}))
		if len(events) != 1 || !strings.Contains(string(events[0].Payload), `"token_count":9007199254740993`) {
			t.Fatalf("Journal arguments changed: %+v", events)
		}
		call := m.toolBatch[0]
		if got := call.args["token_count"]; got != json.Number("9007199254740993") {
			t.Fatalf("approval arguments changed: %#v", got)
		}
		if call.argsJSON != `{"text":"alice@example.com password=synthetic [REDACTED]","token_count":9007199254740993}` {
			t.Fatalf("canonical repeated arguments changed: %s", call.argsJSON)
		}
	}
}

func TestFaithfulNudgeDoesNotCountUnmarkedMatchingOutcomes(t *testing.T) {
	s := newNudgeState()
	for i := 0; i < 3; i++ {
		call := nudgeFailedCall(fmt.Sprintf("matching-%d", i))
		call.Error = ""
		call.Result = "ordinary output matching a refusal"
		if i < 2 {
			call.Failure = nil
		}
		settleBatch(t, s, call)
	}
	if notice, _, err := s.Take(context.Background(), []string{"matching-2"}); err != nil || notice != nil {
		t.Fatalf("unmarked outcomes counted as failed repeats: %+v, %v", notice, err)
	}
}

func TestFaithfulBrokerArgumentsAndDurableResult(t *testing.T) {
	service, backend, runID := newBrokerOperationService(t)
	policy, err := NewPolicyEngine(nil)
	if err != nil {
		t.Fatal(err)
	}
	tool := &brokerTestTool{spec: domain.ToolSpec{Name: "echo_fixture", Params: map[string]domain.ToolParam{"command": {Required: true}, "text": {Required: true}}}}
	text := "alice@example.com password=synthetic sk-test-abcdefghijkl [REDACTED]"
	args := map[string]string{"command": "echo ../; printf '|'", "text": text}
	hook := &faithfulHook{}
	result, err := service.ExecuteBrokerToolOperation(context.Background(), "faithful", tool, policy, NewToolHookChain(time.Second, hook), runID, domain.PolicyProfileFullAuto, args, 8192, false)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result, text) {
		t.Fatalf("result changed: %s", result)
	}
	if !strings.Contains(hook.input, text) || !strings.Contains(hook.output, text) {
		t.Fatalf("hook changed input=%q output=%q", hook.input, hook.output)
	}
	op, err := backend.GetToolOperation(context.Background(), runID, "faithful")
	if err != nil || op.Result != result {
		t.Fatalf("durable result = %q err=%v", op.Result, err)
	}
}

func TestFaithfulHistoryRetainsLiteralData(t *testing.T) {
	text := "alice@example.com token_count=42 password=synthetic [REDACTED]"
	got, redacted, truncated := sanitizeHistoryText(text, 4096)
	if got != text || redacted || truncated {
		t.Fatalf("history = %q redacted=%v truncated=%v", got, redacted, truncated)
	}
}

func TestFaithfulSuccessfulPollingCompletes(t *testing.T) {
	script := append(loopCallScript(8), schema.AssistantMessage("", []schema.ToolCall{{ID: "changed", Function: schema.FunctionCall{Name: "echo_info", Arguments: `{"text":"status changed"}`}}}), schema.AssistantMessage("status changed; complete", nil))
	svc, backend, _ := newLoopGuardService(t, 20, script)
	mustCreateSession(t, backend, "sess-faithful-poll")
	id, err := svc.Run(context.Background(), "sess-faithful-poll", "poll eight times")
	if err != nil {
		t.Fatal(err)
	}
	waitForRunStatus(t, backend, id, domain.RunCompleted)
	events := replayAll(t, backend, id)
	if countToolFinished(events) != 9 || countTerminal(events) != 1 {
		t.Fatal("polling results or terminal ordering changed")
	}
}

func TestFaithfulNudgeFailuresRemainAdvisory(t *testing.T) {
	s := newNudgeState()
	for i := 1; i <= 8; i++ {
		id := fmt.Sprintf("failure-%d", i)
		settleBatch(t, s, nudgeFailedCall(id))
		notice, _, err := s.Take(context.Background(), []string{id})
		if err != nil {
			t.Fatalf("failure %d stopped: %v", i, err)
		}
		if (i == 3 || i == 5) != (notice != nil) {
			t.Fatalf("failure %d notice=%+v", i, notice)
		}
	}
}

func TestFaithfulNudgeSkipsUnfitReminderOnBothModelPaths(t *testing.T) {
	for _, stream := range []bool{false, true} {
		t.Run(fmt.Sprint(stream), func(t *testing.T) {
			s := newNudgeState()
			for i := 1; i <= 3; i++ {
				settleBatch(t, s, nudgeFailedCall(fmt.Sprintf("skip-%d", i)))
			}
			emitted := 0
			ctx := withNudgeEmitter(withNudgeState(context.Background(), s), func(context.Context, nudgeNotice) error { emitted++; return nil })
			input := []*schema.Message{schema.UserMessage("valid input"), {Role: schema.Tool, ToolCallID: "skip-3", Content: "failure"}}
			inner := &recordingInner{}
			w := &nudgeWrappedModel{inner: inner, maxContextBytes: projectedContextBytes(input)}
			for attempt := 0; attempt < 2; attempt++ {
				if stream {
					rd, err := w.Stream(ctx, input)
					if err != nil {
						t.Fatal(err)
					}
					rd.Close()
				} else {
					if _, err := w.Generate(ctx, input); err != nil {
						t.Fatal(err)
					}
				}
				if !reflect.DeepEqual(inner.lastIn, input) || emitted != 0 {
					t.Fatalf("skip changed input or scheduled: emitted=%d input=%+v", emitted, inner.lastIn)
				}
				w.maxContextBytes = 1 << 20
			}
		})
	}
}
