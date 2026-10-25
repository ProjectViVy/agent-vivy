package runtime

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"testing"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/schema"

	"agent-vivy/internal/domain"
)

func provider400(message string) error {
	return errors.New("failed to create chat completion: error, status code: 400, status: 400 Bad Request, message: " + message)
}

func providerStatusError(status int, message string) error {
	return errors.New("failed to create chat completion: error, status code: " +
		strconv.Itoa(status) + ", message: " + message)
}

// The observed rejection: a bare MiniMax 2013 on a child's first call.
func bare2013Error() error {
	return provider400("invalid params, 400 (2013)")
}

func TestBare2013FirstRequestRetriesOnce(t *testing.T) {
	svc := newDecisionService()
	var emitted []GovernanceEvent
	ctx := withGovernanceEventSink(withRunID(context.Background(), "run-bare-2013"),
		func(_ context.Context, event GovernanceEvent) error {
			emitted = append(emitted, event)
			return nil
		})
	input := []*schema.AgenticMessage{testSystem("sys"), testUser("contract"), testUser("task")}

	decision := svc.modelRetryDecision(ctx, &adk.TypedRetryContext[*schema.AgenticMessage]{
		RetryAttempt: 1, Err: bare2013Error(), InputMessages: input,
	})
	if decision == nil || !decision.Retry {
		t.Fatalf("retry not granted for a bare 2013 first request: %+v", decision)
	}
	if decision.RejectReason != provider2013Reason {
		t.Fatalf("reject reason = %v, want %q", decision.RejectReason, provider2013Reason)
	}
	state, ok := svc.modelRetries["run-bare-2013"]
	if !ok || state.reason != provider2013Reason || state.attempt != 1 ||
		state.fingerprint != modelRequestFingerprint(input) {
		t.Fatalf("retry state = %+v", state)
	}
	if svc.autoRetryAwaiting["run-bare-2013"] != 1 {
		t.Fatalf("auto_retry awaiting = %d, want 1", svc.autoRetryAwaiting["run-bare-2013"])
	}
	if len(emitted) != 1 || emitted[0].Type != domain.EventAutoRetryStarted ||
		!strings.Contains(emitted[0].Reason, provider2013Reason) ||
		!strings.Contains(emitted[0].Reason, "code=2013") {
		t.Fatalf("retry events = %+v", emitted)
	}

	// The run has spent its retry: neither the same attempt nor a later one
	// grants another.
	second := svc.provider2013RetryDecision(ctx, &adk.TypedRetryContext[*schema.AgenticMessage]{
		RetryAttempt: 1, Err: bare2013Error(), InputMessages: input,
	})
	if second != nil {
		t.Fatalf("second retry granted: %+v", second)
	}
	beyond := svc.modelRetryDecision(ctx, &adk.TypedRetryContext[*schema.AgenticMessage]{
		RetryAttempt: 2, Err: bare2013Error(), InputMessages: input,
	})
	if beyond != nil {
		t.Fatalf("retry granted beyond the budget: %+v", beyond)
	}
	if len(emitted) != 1 {
		t.Fatalf("second rejection emitted another retry: %+v", emitted)
	}
}

func TestBare2013WithToolTurnDoesNotRetry(t *testing.T) {
	svc := newDecisionService()
	var emitted []GovernanceEvent
	ctx := withGovernanceEventSink(withRunID(context.Background(), "run-bare-2013-tools"),
		func(_ context.Context, event GovernanceEvent) error {
			emitted = append(emitted, event)
			return nil
		})
	input := []*schema.AgenticMessage{
		testUser("start"),
		testToolCalls(testCall("call-a", "echo_info", `{"text":"a"}`)),
		testToolResult("call-a", "echo_info", "a"),
	}

	decision := svc.modelRetryDecision(ctx, &adk.TypedRetryContext[*schema.AgenticMessage]{
		RetryAttempt: 1, Err: bare2013Error(), InputMessages: input,
	})
	if decision != nil {
		t.Fatalf("retry granted despite a tool turn: %+v", decision)
	}
	if len(emitted) != 0 {
		t.Fatalf("emitted retry events: %+v", emitted)
	}
}

func TestBare2013NamedVariantDoesNotRetry(t *testing.T) {
	for _, message := range []string{
		"invalid function arguments json string, tool_call_id: call-a (2013)",
		"context window exceeds limit (2013)",
		"context window exceeds (2013)",
		"maximum context length (2013)",
		"reduce the length (2013)",
		"prompt is too long (2013)",
		"400 request_too_large (2013)",
		"too many tokens (2013)",
		"tool call result does not follow tool call (2013)",
		"400 request validation failed: context_length_exceeded (2013)",
	} {
		svc := newDecisionService()
		var emitted []GovernanceEvent
		ctx := withGovernanceEventSink(withRunID(context.Background(), "run-named-2013"),
			func(_ context.Context, event GovernanceEvent) error {
				emitted = append(emitted, event)
				return nil
			})
		decision := svc.modelRetryDecision(ctx, &adk.TypedRetryContext[*schema.AgenticMessage]{
			RetryAttempt: 1, Err: provider400(message),
			InputMessages: []*schema.AgenticMessage{testUser("task")},
		})
		if decision != nil {
			t.Fatalf("retry granted for %q: %+v", message, decision)
		}
		if len(emitted) != 0 {
			t.Fatalf("retry emitted for %q: %+v", message, emitted)
		}
	}
}

func TestBare2013InvalidInputDoesNotRetry(t *testing.T) {
	svc := newDecisionService()
	var emitted []GovernanceEvent
	ctx := withGovernanceEventSink(withRunID(context.Background(), "run-invalid-first"),
		func(_ context.Context, event GovernanceEvent) error {
			emitted = append(emitted, event)
			return nil
		})
	input := []*schema.AgenticMessage{testUser("")}
	if err := validateModelInput(input); err == nil {
		t.Fatal("fixture does not violate the wire invariants")
	}

	decision := svc.modelRetryDecision(ctx, &adk.TypedRetryContext[*schema.AgenticMessage]{
		RetryAttempt: 1, Err: bare2013Error(), InputMessages: input,
	})
	if decision != nil {
		t.Fatalf("retry granted for an input that fails the wire invariants: %+v", decision)
	}
	if len(emitted) != 0 {
		t.Fatalf("emitted retry events: %+v", emitted)
	}
}

func TestBare2013NonBadRequestDoesNotRetry(t *testing.T) {
	for _, err := range []error{
		providerStatusError(500, "internal error (2013)"),
		providerStatusError(429, "rate limited (2013)"),
		provider400("invalid params"),
		errors.New("2013"),
	} {
		svc := newDecisionService()
		decision := svc.modelRetryDecision(
			withRunID(context.Background(), "run-other"),
			&adk.TypedRetryContext[*schema.AgenticMessage]{
				RetryAttempt: 1, Err: err,
				InputMessages: []*schema.AgenticMessage{testUser("task")},
			})
		if decision != nil {
			t.Fatalf("retry granted for %v: %+v", err, decision)
		}
	}
}

func TestUnambiguousFirstRequestRejectsHistory(t *testing.T) {
	cases := map[string][]*schema.AgenticMessage{
		"assistant text": {testUser("hi"), testAssistant("answer"), testUser("next")},
		"tool call": {
			testUser("hi"),
			testToolCalls(testCall("call-a", "echo_info", `{}`)),
			testToolResult("call-a", "echo_info", "a"),
		},
		"tool result only": {testUser("hi"), testToolResult("call-a", "echo_info", "a")},
		"system only":      {testSystem("sys")},
		"nil message":      {testUser("hi"), nil},
	}
	for name, input := range cases {
		if isUnambiguousFirstRequest(input) {
			t.Fatalf("%s: request treated as an unambiguous first request", name)
		}
	}
	if !isUnambiguousFirstRequest([]*schema.AgenticMessage{
		testSystem("sys"), testUser("contract"), testUser("task"),
	}) {
		t.Fatal("a system+user first request was rejected")
	}
}
