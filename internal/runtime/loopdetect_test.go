package runtime

import (
	"context"
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

// Successful repetition is ordinary authorized work.
func TestServiceToolLoopWithinLimit(t *testing.T) {
	script := append(loopCallScript(5), schema.AssistantMessage("Done repeating.", nil))
	svc, backend, _ := newRepeatedCallService(t, script)
	mustCreateSession(t, backend, "sess-loop-ok")

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
