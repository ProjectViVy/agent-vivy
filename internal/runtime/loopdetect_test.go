package runtime

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/cloudwego/eino/schema"

	"agent-vivy/internal/domain"
)

// countToolFinished counts journaled tool.finished events (inline: the
// shared test helpers only offer first-index helpers).
func countToolFinished(events []domain.RunEvent) int {
	n := 0
	for _, ev := range events {
		if ev.Type == domain.EventToolFinished {
			n++
		}
	}
	return n
}

// A model that repeats the identical call+result signature past the
// window limit is cut off by the repetition guardrail (VC-2, Crush
// StopWhen alignment) with its own cause category.
func TestServiceToolLoopDetected(t *testing.T) {
	// Turn cap kept above the 6 repeats the detector needs, so the
	// repetition guard - not MaxIterations - stops the run.
	svc, backend, _ := newLoopGuardService(t, 20, loopCallScript(8))

	runID, err := svc.Run(context.Background(), "sess-loop-repeat", "echo the same thing")
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	waitForRunStatus(t, backend, runID, domain.RunFailed)

	events := replayAll(t, backend, runID)
	last := events[len(events)-1]
	if last.Type != domain.EventRunFailed {
		t.Fatalf("last event = %s, want run.failed", last.Type)
	}
	cat, msg := payloadFailureOf(t, last.Payload)
	if cat != causeLoopDetected {
		t.Fatalf("cause category = %q, want %q", cat, causeLoopDetected)
	}
	if !strings.Contains(msg, "repeating") {
		t.Fatalf("failure message must name the repetition: %q", msg)
	}
	// Engine internals never reach the user-visible cause (FR-11).
	if strings.Contains(msg, "NodeRunError") || strings.Contains(msg, "fnv") {
		t.Fatalf("failure message leaks internals: %q", msg)
	}
	if n := countTerminal(events); n != 1 {
		t.Fatalf("terminal events = %d, want exactly 1", n)
	}
	// The run looped before detection: five identical results are
	// journaled (the trip batch is discarded with the failure).
	if got := countToolFinished(events); got != 5 {
		t.Fatalf("tool.finished events = %d, want 5", got)
	}
}

// Repeats that stay inside the window limit complete normally: the
// guardrail must not disturb legitimate repeated tool use.
func TestServiceToolLoopWithinLimit(t *testing.T) {
	script := append(loopCallScript(5), schema.AssistantMessage("Done repeating.", nil))
	svc, backend, _ := newLoopGuardService(t, 8, script)

	runID, err := svc.Run(context.Background(), "sess-loop-ok", "echo five times")
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	waitForRunStatus(t, backend, runID, domain.RunCompleted)

	events := replayAll(t, backend, runID)
	last := events[len(events)-1]
	if last.Type != domain.EventRunCompleted {
		t.Fatalf("last event = %s, want run.completed", last.Type)
	}
	if n := countTerminal(events); n != 1 {
		t.Fatalf("terminal events = %d, want exactly 1", n)
	}
}

func TestLoopWindowCountsAndEvicts(t *testing.T) {
	argsA := "{\"text\":\"a\"}"
	argsB := "{\"text\":\"b\"}"

	var w loopWindow
	if err := w.record("echo_info", argsA, "a", ""); err != nil {
		t.Fatalf("first record: %v", err)
	}
	// Window of 10 alternating signatures never exceeds 5 repeats of
	// either, and evicts older entries.
	for i := 0; i < 9; i++ {
		if i%2 == 0 {
			if err := w.record("echo_info", argsB, "b", ""); err != nil {
				t.Fatalf("record %d: %v", i+2, err)
			}
			continue
		}
		if err := w.record("echo_info", argsA, "a", ""); err != nil {
			t.Fatalf("record %d: %v", i+2, err)
		}
	}
	// The 11th record keeps five of each signature in the window; the
	// 12th makes six of the first: the guardrail must trip.
	if err := w.record("echo_info", argsA, "a", ""); err != nil {
		t.Fatalf("record 11: %v", err)
	}
	if err := w.record("echo_info", argsA, "a", ""); !errors.Is(err, errLoopDetected) {
		t.Fatalf("err = %v, want errLoopDetected", err)
	}
	// A different tool name with the same payload is a different call.
	var w2 loopWindow
	for i := 0; i < loopRepeatLimit; i++ {
		if err := w2.record("echo_info", argsA, "a", ""); err != nil {
			t.Fatalf("record %d: %v", i+1, err)
		}
	}
	if err := w2.record("other_tool", argsA, "a", ""); err != nil {
		t.Fatalf("distinct tool flagged: %v", err)
	}
	// Tool errors participate: identical failing calls loop too.
	var w3 loopWindow
	for i := 0; i < loopRepeatLimit; i++ {
		if err := w3.record("echo_info", argsA, "", "boom"); err != nil {
			t.Fatalf("record %d: %v", i+1, err)
		}
	}
	if err := w3.record("echo_info", argsA, "", "boom"); !errors.Is(err, errLoopDetected) {
		t.Fatalf("err = %v, want errLoopDetected", err)
	}
}
