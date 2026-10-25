package runtime

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/schema"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/storage/sqlite"
)

// toolResultOrderErr is the provider rejection shape observed on child runs:
// an HTTP 400 whose body carries the 2013 tool-result-order phrase. A manual
// retry of the identical payload recovers.
const toolResultOrderErr = `error, status code: 400, status: 400 Bad Request, body: {"error":"tool call result does not follow tool call (2013)"}`

// orderToolTurn is the scripted first answer: the run must actually carry a
// tool call and its result, otherwise the retry gate refuses by design.
func orderToolTurn() *schema.AgenticMessage {
	return testToolCalls(testCall("order-call-1", "echo_info", `{"text":"probe"}`))
}

func TestToolResultOrderRetryRecoversRun(t *testing.T) {
	m := &overflowScriptedModel{
		skipFirst: 1, failCount: 1, errText: toolResultOrderErr,
		responses: map[int]*schema.AgenticMessage{1: orderToolTurn()},
	}
	svc, backend := newOverflowService(t, m)
	ctx := context.Background()
	mustCreateSession(t, backend, "sess-order")

	runID, err := svc.Run(ctx, "sess-order", "order me")
	if err != nil {
		t.Fatalf("run admission: %v", err)
	}
	waitFor(t, "run completes after retry", func() bool {
		return runTerminal(t, backend, runID) == domain.RunCompleted
	})

	if got := m.calls(); got != 3 {
		t.Fatalf("model calls = %d, want 3 (tool turn, rejected call, retried call)", got)
	}
	if got := m.inputFingerprint(2); got == "" || got != m.inputFingerprint(3) {
		t.Fatalf("retry input fingerprint differs from the rejected input:\n%q\n%q", got, m.inputFingerprint(3))
	}

	events := journalEvents(t, backend, runID)
	started := eventsOfType(events, domain.EventAutoRetryStarted)
	if len(started) != 1 {
		t.Fatalf("auto_retry.started count = %d, want 1", len(started))
	}
	reason, _ := eventPayload(t, started[0])["reason"].(string)
	if !strings.Contains(reason, toolResultOrderReason) || !strings.Contains(reason, "fingerprint=") {
		t.Fatalf("auto_retry.started reason = %v", reason)
	}
	finished := eventsOfType(events, domain.EventAutoRetryFinished)
	if len(finished) != 1 || eventPayload(t, finished[0])["success"] != true {
		t.Fatalf("auto_retry.finished events = %v", finished)
	}
	if got := len(eventsOfType(events, domain.EventContextCompacted)); got != 0 {
		t.Fatalf("context.compacted count = %d, want 0 (no compaction for this fault)", got)
	}

	// The two v5 request rows are the journal proof that the retry replayed
	// the same structural input.
	requests := modelRequestPayloadsV5(t, events)
	if len(requests) != 3 {
		t.Fatalf("model.request rows = %d, want 3", len(requests))
	}
	assertSameRequestShape(t, requests[1], requests[2])
	if len(requests[1].ResultCallIDs) != 1 || requests[1].ResultCallIDs[0] != "order-call-1" {
		t.Fatalf("rejected request result ids = %v", requests[1].ResultCallIDs)
	}
}

func TestToolResultOrderSecondFailureEndsRun(t *testing.T) {
	// The retry is rejected the same way: one recovery per run, then honest
	// failure instead of replaying the payload in a loop.
	m := &overflowScriptedModel{
		skipFirst: 1, failCount: 2, errText: toolResultOrderErr,
		responses: map[int]*schema.AgenticMessage{1: orderToolTurn()},
	}
	svc, backend := newOverflowService(t, m)
	ctx := context.Background()
	mustCreateSession(t, backend, "sess-order2")

	runID, err := svc.Run(ctx, "sess-order2", "order me twice")
	if err != nil {
		t.Fatalf("run admission: %v", err)
	}
	waitFor(t, "run fails", func() bool {
		return runTerminal(t, backend, runID) == domain.RunFailed
	})

	events := journalEvents(t, backend, runID)
	if got := len(eventsOfType(events, domain.EventAutoRetryStarted)); got != 1 {
		t.Fatalf("auto_retry.started count = %d, want exactly 1 (no retry loop)", got)
	}
	finished := eventsOfType(events, domain.EventAutoRetryFinished)
	if len(finished) != 1 || eventPayload(t, finished[0])["success"] != false {
		t.Fatalf("auto_retry.finished events = %v", finished)
	}
	if len(eventsOfType(events, domain.EventRunFailed)) == 0 {
		t.Fatalf("missing run.failed terminal")
	}

	requests := modelRequestPayloadsV5(t, events)
	if len(requests) != 3 {
		t.Fatalf("model.request rows = %d, want 3", len(requests))
	}
	assertSameRequestShape(t, requests[1], requests[2])
}

func TestOverflowRetryConsumesBudgetBeforeToolResultOrder(t *testing.T) {
	// One physical retry per run: overflow recovery takes it, so a later
	// tool-result-order rejection must not publish a retry that cannot happen.
	// The run carries a real tool turn (seeded history) so the order gate is
	// evaluated on the budget, not on the missing-tool-turn shortcut.
	m := &overflowScriptedModel{
		skipFirst: 0,
		failCount: 1,
		errText:   "prompt is too long",
		errByCall: map[int]string{2: toolResultOrderErr},
	}
	svc, backend := newOverflowService(t, m)
	ctx := context.Background()
	mustCreateSession(t, backend, "sess-order3")
	seedOrderToolTurn(t, backend, "sess-order3")

	runID, err := svc.Run(ctx, "sess-order3", "overflow then order")
	if err != nil {
		t.Fatalf("run admission: %v", err)
	}
	waitFor(t, "run fails", func() bool {
		return runTerminal(t, backend, runID) == domain.RunFailed
	})

	events := journalEvents(t, backend, runID)
	started := eventsOfType(events, domain.EventAutoRetryStarted)
	if len(started) != 1 {
		t.Fatalf("auto_retry.started count = %d, want 1", len(started))
	}
	if r := eventPayload(t, started[0])["reason"]; !strings.Contains(r.(string), overflowRecoveryReason) {
		t.Fatalf("auto_retry.started reason = %v, want the overflow recovery", r)
	}
	if got := len(eventsOfType(events, domain.EventContextCompacted)); got != 1 {
		t.Fatalf("context.compacted count = %d, want 1", got)
	}
	finished := eventsOfType(events, domain.EventAutoRetryFinished)
	if len(finished) != 1 || eventPayload(t, finished[0])["success"] != false {
		t.Fatalf("auto_retry.finished events = %v", finished)
	}
}

// seedOrderToolTurn persists a completed assistant tool call and its result as
// session history, so the next run's input carries a real tool turn.
func seedOrderToolTurn(t *testing.T, backend *sqlite.Backend, sessionID domain.SessionID) {
	t.Helper()
	ctx := context.Background()
	rows := []domain.Message{
		{ID: "seed-call", SessionID: sessionID, Role: domain.RoleAssistant,
			ToolCallID: "seed-tool-call", ToolName: "echo_info", ToolArgs: []byte(`{"text":"probe"}`), CreatedAt: 1},
		{ID: "seed-result", SessionID: sessionID, Role: domain.RoleTool,
			ToolCallID: "seed-tool-call", ToolName: "echo_info", Content: "probe", CreatedAt: 2},
	}
	for _, row := range rows {
		if err := backend.AppendMessage(ctx, row); err != nil {
			t.Fatalf("append seeded tool turn: %v", err)
		}
	}
}

func TestToolResultOrderRetryRequiresToolTurn(t *testing.T) {
	var emitted []GovernanceEvent
	svc := newDecisionService()
	ctx := withGovernanceEventSink(withRunID(context.Background(), "run-no-tool-turn"),
		func(_ context.Context, event GovernanceEvent) error {
			emitted = append(emitted, event)
			return nil
		})
	decision := svc.toolResultOrderRetryDecision(ctx, &adk.TypedRetryContext[*schema.AgenticMessage]{
		RetryAttempt:  1,
		Err:           errors.New(toolResultOrderErr),
		InputMessages: []*schema.AgenticMessage{testUser("no tools here")},
	})
	if decision != nil {
		t.Fatalf("retry granted without a tool turn: %+v", decision)
	}
	if len(emitted) != 0 {
		t.Fatalf("emitted retry events without a tool turn: %+v", emitted)
	}
}

func TestToolResultOrderRetryDeclinesInvalidInput(t *testing.T) {
	var emitted []GovernanceEvent
	svc := newDecisionService()
	ctx := withGovernanceEventSink(withRunID(context.Background(), "run-invalid-input"),
		func(_ context.Context, event GovernanceEvent) error {
			emitted = append(emitted, event)
			return nil
		})
	// A result with no preceding call satisfies hasToolTurns but not the wire
	// invariants, so replaying it would only repeat the rejection.
	input := []*schema.AgenticMessage{
		testUser("hi"),
		testToolResult("missing-call", "echo_info", "orphan"),
	}
	if err := validateModelInput(input); err == nil {
		t.Fatal("fixture does not violate the wire invariants")
	}
	decision := svc.toolResultOrderRetryDecision(ctx, &adk.TypedRetryContext[*schema.AgenticMessage]{
		RetryAttempt: 1, Err: errors.New(toolResultOrderErr), InputMessages: input,
	})
	if decision != nil {
		t.Fatalf("retry granted for an invalid input: %+v", decision)
	}
	if len(emitted) != 0 {
		t.Fatalf("emitted retry events for an invalid input: %+v", emitted)
	}
}

func TestToolResultOrderRetryFingerprintIdentity(t *testing.T) {
	var emitted []GovernanceEvent
	svc := newDecisionService()
	ctx := withGovernanceEventSink(withRunID(context.Background(), "run-fingerprint"),
		func(_ context.Context, event GovernanceEvent) error {
			emitted = append(emitted, event)
			return nil
		})
	input := []*schema.AgenticMessage{
		testUser("hi"),
		testToolCalls(testCall("call-a", "echo_info", `{"text":"a"}`)),
		testToolResult("call-a", "echo_info", "a"),
	}
	decision := svc.toolResultOrderRetryDecision(ctx, &adk.TypedRetryContext[*schema.AgenticMessage]{
		RetryAttempt: 1, Err: errors.New(toolResultOrderErr), InputMessages: input,
	})
	if decision == nil || !decision.Retry {
		t.Fatalf("retry not granted for a valid tool turn: %+v", decision)
	}
	want := modelRequestFingerprint(input)
	state, ok := svc.toolOrderRetries["run-fingerprint"]
	if !ok || state.fingerprint != want || state.attempt != 1 {
		t.Fatalf("stored retry state = %+v, want fingerprint %q", state, want)
	}
	if svc.autoRetryAwaiting["run-fingerprint"] != 1 {
		t.Fatalf("auto_retry awaiting = %d, want 1", svc.autoRetryAwaiting["run-fingerprint"])
	}
	if len(emitted) != 1 || emitted[0].Type != domain.EventAutoRetryStarted ||
		!strings.Contains(emitted[0].Reason, "fingerprint="+clampFingerprint(want)) {
		t.Fatalf("retry events = %+v", emitted)
	}

	// A second rejection never retries and is compared against the first
	// fingerprint.
	second := svc.toolResultOrderRetryDecision(ctx, &adk.TypedRetryContext[*schema.AgenticMessage]{
		RetryAttempt: 2, Err: errors.New(toolResultOrderErr), InputMessages: input,
	})
	if second != nil {
		t.Fatalf("second retry granted: %+v", second)
	}
	if len(emitted) != 1 {
		t.Fatalf("second rejection emitted another retry: %+v", emitted)
	}
}

func newDecisionService() *Service {
	return &Service{
		toolOrderRetries:  map[domain.RunID]toolOrderRetryState{},
		autoRetryAwaiting: map[domain.RunID]int{},
	}
}

// modelRequestPayloadsV5 decodes every model.request row and requires the
// current envelope version.
func modelRequestPayloadsV5(t *testing.T, events []domain.RunEvent) []payloadModelRequestV5 {
	t.Helper()
	var out []payloadModelRequestV5
	for _, event := range events {
		if event.Type != domain.EventModelRequest {
			continue
		}
		if event.PayloadVersion != 5 {
			t.Fatalf("model.request payload version = %d, want 5", event.PayloadVersion)
		}
		var payload payloadModelRequestV5
		if err := json.Unmarshal(event.Payload, &payload); err != nil {
			t.Fatalf("decode model.request v5: %v", err)
		}
		out = append(out, payload)
	}
	return out
}

func assertSameRequestShape(t *testing.T, first, second payloadModelRequestV5) {
	t.Helper()
	if !reflect.DeepEqual(first.Messages, second.Messages) {
		t.Fatalf("request messages differ:\n%+v\n%+v", first.Messages, second.Messages)
	}
	if !reflect.DeepEqual(first.CallIDs, second.CallIDs) ||
		!reflect.DeepEqual(first.ResultCallIDs, second.ResultCallIDs) ||
		!reflect.DeepEqual(first.ToolDeclarations, second.ToolDeclarations) ||
		first.Counts != second.Counts {
		t.Fatalf("request structural fields differ:\n%+v\n%+v", first, second)
	}
}

func TestIsToolResultOrderFailureClassifier(t *testing.T) {
	cases := []struct {
		err  string
		want bool
	}{
		{toolResultOrderErr, true},
		{`error, status code: 400, status: 400 Bad Request, body: {"error":"invalid params, 400 (2013)"}`, false},
		{`error, status code: 400, status: 400 Bad Request, body: {"error":"bad request"}`, false},
		{`error, status code: 500, body: {"error":"tool call result does not follow tool call (2013)"}`, false},
		{"maximum context length is 8192 tokens", false},
		{"connection refused", false},
	}
	for _, tc := range cases {
		if got := isToolResultOrderFailure(errors.New(tc.err)); got != tc.want {
			t.Errorf("isToolResultOrderFailure(%q) = %v, want %v", tc.err, got, tc.want)
		}
	}
	if isToolResultOrderFailure(nil) {
		t.Errorf("nil error classified as a tool-result-order failure")
	}
}
