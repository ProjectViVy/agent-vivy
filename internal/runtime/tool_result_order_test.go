package runtime

import (
	"context"
	"errors"
	"strings"
	"testing"

	"agent-vivy/internal/domain"
)

// toolResultOrderErr is the provider rejection shape observed on child runs:
// an HTTP 400 whose body carries the 2013 tool-result-order code. A manual
// retry of the identical payload recovers.
const toolResultOrderErr = `error, status code: 400, status: 400 Bad Request, body: {"error":"tool call result does not follow tool call (2013)"}`

func TestToolResultOrderRetryRecoversRun(t *testing.T) {
	m := &overflowScriptedModel{skipFirst: 0, failCount: 1, errText: toolResultOrderErr}
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

	if got := m.calls(); got != 2 {
		t.Fatalf("model calls = %d, want 2 (rejected, retried)", got)
	}
	// The retry replays the identical payload: this fault is the provider's,
	// not the input's, so nothing is rewritten.
	first, second := m.inputTexts(1), m.inputTexts(2)
	if len(first) == 0 || strings.Join(first, "\n") != strings.Join(second, "\n") {
		t.Fatalf("retry input differs from the rejected input:\n%v\n%v", first, second)
	}

	events := journalEvents(t, backend, runID)
	started := eventsOfType(events, domain.EventAutoRetryStarted)
	if len(started) != 1 {
		t.Fatalf("auto_retry.started count = %d, want 1", len(started))
	}
	if r := eventPayload(t, started[0])["reason"]; !strings.Contains(r.(string), toolResultOrderReason) {
		t.Fatalf("auto_retry.started reason = %v", r)
	}
	finished := eventsOfType(events, domain.EventAutoRetryFinished)
	if len(finished) != 1 || eventPayload(t, finished[0])["success"] != true {
		t.Fatalf("auto_retry.finished events = %v", finished)
	}
	if got := len(eventsOfType(events, domain.EventContextCompacted)); got != 0 {
		t.Fatalf("context.compacted count = %d, want 0 (no compaction for this fault)", got)
	}
}

func TestToolResultOrderSecondFailureEndsRun(t *testing.T) {
	// The retry is rejected the same way: one recovery per run, then honest
	// failure instead of replaying the payload in a loop.
	m := &overflowScriptedModel{skipFirst: 0, failCount: 2, errText: toolResultOrderErr}
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
}

func TestOverflowRetryConsumesBudgetBeforeToolResultOrder(t *testing.T) {
	// One physical retry per run: overflow recovery takes it, so a later
	// tool-result-order rejection must not publish a retry that cannot happen.
	m := &overflowScriptedModel{
		skipFirst: 1,
		failCount: 2,
		errText:   "prompt is too long",
		errByCall: map[int]string{3: toolResultOrderErr},
	}
	svc, backend := newOverflowService(t, m)
	ctx := context.Background()
	mustCreateSession(t, backend, "sess-order3")

	seedID, err := svc.Run(ctx, "sess-order3", "seed")
	if err != nil {
		t.Fatalf("seed run: %v", err)
	}
	waitFor(t, "seed run completes", func() bool {
		return runTerminal(t, backend, seedID) == domain.RunCompleted
	})
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

func TestIsToolResultOrderFailureClassifier(t *testing.T) {
	cases := []struct {
		err  string
		want bool
	}{
		{toolResultOrderErr, true},
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
