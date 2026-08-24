package runtime

import (
	"context"
	"strings"
	"testing"

	"agent-vivy/internal/tools"
)

func TestEinoSequentialThinkingBackendScopesAndValidatesRuns(t *testing.T) {
	backend := NewEinoSequentialThinkingBackend()
	first, err := backend.Think(context.Background(), "run-a", tools.SequentialThoughtRequest{Thought: "first", ThoughtNumber: 1, TotalThoughts: 2, NextThoughtNeeded: true})
	if err != nil || !first.Accepted || first.ThoughtNumber != 1 {
		t.Fatalf("first=%#v err=%v", first, err)
	}
	second, err := backend.Think(context.Background(), "run-a", tools.SequentialThoughtRequest{Thought: "second", ThoughtNumber: 2, TotalThoughts: 2})
	if err != nil || !second.Accepted || second.TotalThoughts != 2 {
		t.Fatalf("second=%#v err=%v", second, err)
	}
	revision, err := backend.Think(context.Background(), "run-a", tools.SequentialThoughtRequest{Thought: "revised", ThoughtNumber: 2, TotalThoughts: 2, IsRevision: true, RevisesThought: 2})
	if err != nil || !revision.IsRevision {
		t.Fatalf("revision=%#v err=%v", revision, err)
	}
	if _, err := backend.Think(context.Background(), "run-b", tools.SequentialThoughtRequest{Thought: "skip", ThoughtNumber: 2, TotalThoughts: 2}); err == nil || !strings.Contains(err.Error(), "expected thought_number") {
		t.Fatalf("out of order error=%v", err)
	}
	if _, err := backend.Think(context.Background(), "run-a", tools.SequentialThoughtRequest{Thought: "bad", ThoughtNumber: 4, TotalThoughts: 4}); err == nil {
		t.Fatal("expected sequence gap error")
	}
}
func TestEinoSequentialThinkingBackendBoundsThoughtSize(t *testing.T) {
	backend := NewEinoSequentialThinkingBackend()
	_, err := backend.Think(context.Background(), "run", tools.SequentialThoughtRequest{Thought: strings.Repeat("x", maxThoughtBytes+1), ThoughtNumber: 1, TotalThoughts: 1})
	if err == nil || !strings.Contains(err.Error(), "exceeds") {
		t.Fatalf("size error=%v", err)
	}
}
