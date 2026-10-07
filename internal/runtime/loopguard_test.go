package runtime

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

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

func newRepeatedCallService(t *testing.T, script []*schema.Message) (*Service, *sqlite.Backend, *testSink) {
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
		StreamBuffer: 8, MaxEventPayloadBytes: 64 << 10,
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

func TestServiceRepeatedCallsComplete(t *testing.T) {
	script := append(loopCallScript(2), schema.AssistantMessage("Done spinning.", nil))
	svc, backend, _ := newRepeatedCallService(t, script)

	mustCreateSession(t, backend, "sess-ok")
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

func TestServiceWithoutTurnPolicyExceedsFormerLimits(t *testing.T) {
	for _, child := range []bool{false, true} {
		t.Run(fmt.Sprintf("child=%v", child), func(t *testing.T) {
			script := append(loopCallScript(24), schema.AssistantMessage("Completed beyond twenty turns.", nil))
			svc, backend, sink := newRepeatedCallService(t, script)
			if child {
				engine, err := svc.engine.ChildView(context.Background(), []string{tools.EchoInfoName})
				if err != nil {
					t.Fatal(err)
				}
				svc = NewService(engine, "scripted", "scripted-v0", ServiceDeps{
					Journal: backend, Runs: backend, Messages: backend, Notes: backend, Sink: sink,
				})
			}
			t.Cleanup(func() { svc.CancelAll(); svc.WaitIdle(context.Background()) })
			mustCreateSession(t, backend, "sess-long-run")
			id, err := svc.Run(context.Background(), "sess-long-run", "perform twenty-four calls and finish")
			if err != nil {
				t.Fatal(err)
			}
			waitForRunStatus(t, backend, id, domain.RunCompleted)
			events := replayAll(t, backend, id)
			if countToolFinished(events) != 24 || countTerminal(events) != 1 {
				t.Fatalf("lost results or duplicate terminal: finished=%d terminals=%d", countToolFinished(events), countTerminal(events))
			}
		})
	}
}

func TestServiceBudgetCircuitBreakerStopsToolTree(t *testing.T) {
	svc, backend, _ := newRepeatedCallService(t, loopCallScript(4))
	svc.deps.Budget = BudgetPolicy{MaxToolCalls: 1}

	mustCreateSession(t, backend, "sess-budget")
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

func TestServiceModelBudgetStillStopsLongRuns(t *testing.T) {
	svc, backend, _ := newRepeatedCallService(t, loopCallScript(24))
	svc.deps.Budget = BudgetPolicy{MaxModelCalls: 22}
	mustCreateSession(t, backend, "sess-long-budget")
	id, err := svc.Run(context.Background(), "sess-long-budget", "perform twenty-four calls")
	if err != nil {
		t.Fatal(err)
	}
	waitForRunStatus(t, backend, id, domain.RunFailed)
	events := replayAll(t, backend, id)
	last := events[len(events)-1]
	category, message := payloadFailureOf(t, last.Payload)
	if category != causeInternalError || !strings.Contains(message, "safety budget") {
		t.Fatalf("long-run failure = %s/%q, want shared safety budget", category, message)
	}
	if countToolFinished(events) != 22 || countTerminal(events) != 1 {
		t.Fatalf("budget admission or terminal ordering changed: finished=%d terminals=%d", countToolFinished(events), countTerminal(events))
	}
}

func TestServiceLongRunRemainsCancellable(t *testing.T) {
	entered := make(chan struct{})
	var calls atomic.Int32
	tool := newContractTool(func(ctx context.Context, _ json.RawMessage) (string, error) {
		if calls.Add(1) == 25 {
			close(entered)
			<-ctx.Done()
			return "", ctx.Err()
		}
		return "ordinary status", nil
	})
	var script []*schema.Message
	for i := 0; i < 25; i++ {
		script = append(script, schema.AssistantMessage("", []schema.ToolCall{contractToolCall(fmt.Sprintf("cancel-long-%d", i), "status")}))
	}
	h := newAcceptanceHarness(t, script, acceptanceOpts{extraTools: []tools.Tool{tool}})
	mustCreateSession(t, h.backend, "sess-long-cancel")
	id, err := h.svc.Run(context.Background(), "sess-long-cancel", "keep reading status")
	if err != nil {
		t.Fatal(err)
	}
	select {
	case <-entered:
	case <-time.After(5 * time.Second):
		t.Fatal("run did not reach the twenty-fifth call")
	}
	if !h.svc.Cancel(id) {
		t.Fatal("long Run could not be cancelled")
	}
	waitForRunStatus(t, h.backend, id, domain.RunCancelled)
	events := replayAll(t, h.backend, id)
	if countToolFinished(events) < 24 || countTerminal(events) != 1 || events[len(events)-1].Type != domain.EventRunCancelled {
		t.Fatalf("long-run cancellation lost settlement or terminal ordering: %+v", events[len(events)-1])
	}
}
