package runtime

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cloudwego/eino/schema"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/storage/sqlite"
	"agent-vivy/internal/tools"
)

// loopCallScript builds a script of n read-only echo_info tool calls
// with distinct call ids: a model that keeps calling tools forever.
func loopCallScript(n int) []*schema.Message {
	script := make([]*schema.Message, 0, n)
	for i := 0; i < n; i++ {
		script = append(script, schema.AssistantMessage("", []schema.ToolCall{{
			ID:       fmt.Sprintf("call-loop-%d", i),
			Function: schema.FunctionCall{Name: tools.EchoInfoName, Arguments: `{"text":"spin"}`},
		}}))
	}
	return script
}

// newLoopGuardService wires an engine with an explicit tool-turn cap
// over the auto-execute echo tool — no approval gate is involved, so the
// loop runs until the guardrail stops it (MA-4).
func newLoopGuardService(t *testing.T, maxToolTurns int, script []*schema.Message) (*Service, *sqlite.Backend, *testSink) {
	t.Helper()
	ctx := context.Background()

	backend, err := sqlite.Open(ctx, filepath.Join(t.TempDir(), "loop.db"))
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { _ = backend.Close() })

	ts, err := tools.Builtin(backend).Resolve([]string{tools.EchoInfoName})
	if err != nil {
		t.Fatalf("resolve tools: %v", err)
	}
	eng, err := NewEngine(ctx, NewScriptedModel(script...), ts, EngineConfig{
		StreamBuffer: 8, MaxEventPayloadBytes: 64 << 10, MaxToolTurns: maxToolTurns,
	})
	if err != nil {
		t.Fatalf("new engine: %v", err)
	}
	sink := newTestSink()
	svc := NewService(eng, "scripted", "scripted-v0", ServiceDeps{
		Journal: backend, Runs: backend, Messages: backend, Notes: backend, Sink: sink,
	})
	return svc, backend, sink
}

// A model that never stops calling tools must be cut off by the cap and
// close as a definitively failed run with a bounded, classified cause.
func TestServiceMaxToolTurnsBreached(t *testing.T) {
	svc, backend, _ := newLoopGuardService(t, 2, loopCallScript(6))

	runID, err := svc.Run(context.Background(), "sess-loop", "echo round")
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
	if cat != causeInternalError {
		t.Fatalf("cause category = %q, want %q", cat, causeInternalError)
	}
	if !strings.Contains(msg, "tool-call turns") {
		t.Fatalf("failure message must name the guardrail: %q", msg)
	}
	// Engine internals never reach the user-visible cause (FR-11).
	if strings.Contains(msg, "exceeds max iterations") || strings.Contains(msg, "NodeRunError") {
		t.Fatalf("failure message leaks engine internals: %q", msg)
	}
	if n := countTerminal(events); n != 1 {
		t.Fatalf("terminal events = %d, want exactly 1", n)
	}
	// The loop did run before the cap: tool events are journaled.
	if indexOfType(events, domain.EventToolFinished) < 0 {
		var types []string
		for _, ev := range events {
			types = append(types, string(ev.Type))
		}
		t.Fatalf("expected tool executions before the cap, got %v", types)
	}
}

// Runs that stay inside the cap complete normally: the guardrail must
// not disturb healthy tool flows.
func TestServiceMaxToolTurnsWithinCap(t *testing.T) {
	script := append(loopCallScript(2), schema.AssistantMessage("Done spinning.", nil))
	svc, backend, _ := newLoopGuardService(t, 8, script)

	runID, err := svc.Run(context.Background(), "sess-ok", "echo spin twice")
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

func TestServiceBudgetCircuitBreakerStopsToolTree(t *testing.T) {
	svc, backend, _ := newLoopGuardService(t, 8, loopCallScript(4))
	// Keep the Eino iteration cap permissive and trip Vivy's shared tool
	// budget instead. A resumed/child scope would spend this same account.
	svc.deps.Budget = BudgetPolicy{MaxToolCalls: 1}

	runID, err := svc.Run(context.Background(), "sess-budget", "echo repeatedly")
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	waitForRunStatus(t, backend, runID, domain.RunFailed)

	events := replayAll(t, backend, runID)
	last := events[len(events)-1]
	cat, msg := payloadFailureOf(t, last.Payload)
	if last.Type != domain.EventRunFailed || cat != causeInternalError {
		t.Fatalf("terminal = %s/%q, want internal run.failed", last.Type, cat)
	}
	if !strings.Contains(msg, "safety budget") {
		t.Fatalf("failure message = %q, want bounded budget wording", msg)
	}
	if n := countTerminal(events); n != 1 {
		t.Fatalf("terminal events = %d, want exactly 1", n)
	}
}
