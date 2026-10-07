package runtime

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/tools"
	"github.com/cloudwego/eino/schema"
)

type faithfulHook struct{ input, output string }

func (*faithfulHook) Name() string { return "faithful" }
func (hook *faithfulHook) PreToolUse(_ context.Context, call ToolHookCall) (PreToolUseResult, error) {
	hook.input = string(call.Arguments)
	return PreToolUseResult{Decision: hookAllow}, nil
}
func (hook *faithfulHook) PostToolUse(_ context.Context, _ ToolHookCall, result string, _ error) error {
	hook.output = result
	return nil
}

func TestFaithfulRepeatedFailuresAndRefusalsCompleteThroughService(t *testing.T) {
	for _, refused := range []bool{false, true} {
		t.Run(fmt.Sprint(refused), func(t *testing.T) {
			var script []*schema.Message
			denied := &classifierStubTool{class: tools.InvocationDenied, findings: []string{"forbidden"}}
			for i := 1; i <= 8; i++ {
				call := contractToolCall(fmt.Sprintf("repeat-%d", i), "same")
				if refused {
					call.Function = schema.FunctionCall{Name: "stub_classifier", Arguments: `{"command":"x"}`}
				}
				script = append(script, schema.AssistantMessage("", []schema.ToolCall{call}))
			}
			script = append(script, schema.AssistantMessage("done", nil))
			tool := tools.Tool(failingContractTool())
			if refused {
				tool = denied
			}
			h := newContractHarness(t, script, contractHarnessOpts{streaming: true, noBarrier: true, prodNudge: true, extraTools: []tools.Tool{tool}})
			id, err := h.svc.Run(context.Background(), "sess-repeat", "repeat authorized attempts")
			if err != nil {
				t.Fatal(err)
			}
			waitForRunStatus(t, h.backend, id, domain.RunCompleted)
			nudges := nudgeEvents(t, h.journal)
			if len(nudges) != 2 || nudges[0].RepeatCount != 3 || nudges[1].RepeatCount != 5 {
				t.Fatalf("nudges=%+v", nudges)
			}
			if countToolFinished(replayAll(t, h.backend, id)) != 8 || denied.calls != 0 {
				t.Fatal("lost results or denied tool executed")
			}
		})
	}
}

func TestFaithfulToolModelAndReplayPreserveSyntheticData(t *testing.T) {
	text := "alice@example.com password=synthetic sk-test-abcdefghijkl [REDACTED] token_count=42"
	for _, enhanced := range []bool{false, true} {
		t.Run(fmt.Sprint(enhanced), func(t *testing.T) {
			script := []*schema.Message{schema.AssistantMessage("", []schema.ToolCall{contractToolCall("faithful-model", text)}), schema.AssistantMessage(text, nil)}
			tool := newContractTool(func(_ context.Context, args json.RawMessage) (string, error) {
				var fields map[string]string
				err := json.Unmarshal(args, &fields)
				return fields["text"], err
			})
			h := newContractHarness(t, script, contractHarnessOpts{streaming: true, noBarrier: true, prodNudge: true, enhanced: enhanced, extraTools: []tools.Tool{tool}})
			id, err := h.svc.Run(context.Background(), "sess-faithful-model", text)
			if err != nil {
				t.Fatal(err)
			}
			waitForRunStatus(t, h.backend, id, domain.RunCompleted)
			input := h.model.entry(1).input
			found := false
			for _, msg := range input {
				if msg.Role == schema.Tool && strings.Contains(msg.Content, text) {
					found = true
				}
				if msg.Role == schema.Tool {
					for _, part := range msg.UserInputMultiContent {
						if strings.Contains(part.Text, text) {
							found = true
						}
					}
				}
			}
			if !found {
				t.Fatalf("model lost tool text: %+v", input)
			}
			finished, _ := json.Marshal(acceptanceFinished(t, replayAll(t, h.backend, id), "faithful-model"))
			if !strings.Contains(string(finished), text) {
				t.Fatal("replay lost task data")
			}
		})
	}
}

func TestFaithfulNudgeFittingReminderIsIdenticalOnRetry(t *testing.T) {
	for _, stream := range []bool{false, true} {
		t.Run(fmt.Sprint(stream), func(t *testing.T) {
			s := newNudgeState()
			for i := 1; i <= 3; i++ {
				settleBatch(t, s, nudgeFailedCall(fmt.Sprintf("fit-%d", i)))
			}
			emitted := 0
			ctx := withNudgeEmitter(withNudgeState(context.Background(), s), func(context.Context, nudgeNotice) error { emitted++; return nil })
			input := []*schema.Message{schema.UserMessage("valid"), {Role: schema.Tool, ToolCallID: "fit-3", Content: "failure"}}
			inner := &recordingInner{}
			w := &nudgeWrappedModel{inner: inner, maxContextBytes: 1 << 20}
			var first []*schema.Message
			for attempt := 0; attempt < 2; attempt++ {
				if stream {
					r, err := w.Stream(ctx, input)
					if err != nil {
						t.Fatal(err)
					}
					r.Close()
				} else {
					if _, err := w.Generate(ctx, input); err != nil {
						t.Fatal(err)
					}
				}
				if emitted != 1 || len(nudgeMessages(inner.lastIn)) != 1 {
					t.Fatalf("emitted=%d input=%+v", emitted, inner.lastIn)
				}
				if attempt == 0 {
					first = inner.lastIn
				} else if !reflect.DeepEqual(first, inner.lastIn) {
					t.Fatal("retry changed reminder")
				}
			}
		})
	}
}

func TestFaithfulNudgeJournalFailureCannotBeRetriedIntoUnrecordedInjection(t *testing.T) {
	state := newNudgeState()
	for i := 1; i <= 3; i++ {
		settleBatch(t, state, nudgeFailedCall(fmt.Sprintf("journal-%d", i)))
	}
	cause := errors.New("injected scheduling journal failure")
	ctx := withNudgeEmitter(withNudgeState(context.Background(), state), func(context.Context, nudgeNotice) error { return cause })
	input := []*schema.Message{{Role: schema.Tool, ToolCallID: "journal-3", Content: "failure"}}
	inner := &recordingInner{}
	wrapped := &nudgeWrappedModel{inner: inner, maxContextBytes: 1 << 20}
	for attempt := 0; attempt < 2; attempt++ {
		_, err := wrapped.Generate(ctx, input)
		if !errors.Is(err, cause) || inner.lastIn != nil {
			t.Fatalf("attempt %d passed undurable reminder: err=%v input=%+v", attempt, err, inner.lastIn)
		}
	}
}
