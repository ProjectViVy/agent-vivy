package runtime

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"testing"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/storage/sqlite"
	"agent-vivy/internal/tools"
	"github.com/cloudwego/eino/schema"
)

func nudgeTestFailure() *toolFailure {
	return &toolFailure{
		Status:     "recoverable",
		Reason:     "remote_tool_error",
		Diagnostic: "tool failed: upstream refused",
		Effects:    "none",
	}
}

// nudgeFailedCall is one identical unsuccessful singleton outcome.
func nudgeFailedCall(id string) completedCall {
	return completedCall{
		ID:       id,
		Name:     "echo_info",
		ArgsJSON: `{"text":"spin"}`,
		Result:   "",
		Error:    "boom",
		Failure:  nudgeTestFailure(),
	}
}

// nudgeOkCall is the same call succeeding: identical name+args, no
// failure mark and no error text.
func nudgeOkCall(id string) completedCall {
	return completedCall{
		ID:       id,
		Name:     "echo_info",
		ArgsJSON: `{"text":"spin"}`,
		Result:   "ok",
	}
}

// settleBatch exercises the same synchronous native boundary as WrapModel,
// leaving the notice untaken so the test can inspect its scheduling signal.
func settleBatch(t *testing.T, s *nudgeState, calls ...completedCall) {
	t.Helper()
	ids := make([]string, 0, len(calls))
	for _, call := range calls {
		if err := s.record(call, false, true); err != nil {
			t.Fatal(err)
		}
		ids = append(ids, call.ID)
	}
	if err := s.prepare(context.Background(), ids); err != nil && !errors.Is(err, errLoopDetected) {
		t.Fatal(err)
	}
}

func TestNudgeStateRepetitionReminders(t *testing.T) {
	s := newNudgeState()
	for i := 1; i <= 6; i++ {
		id := fmt.Sprintf("call-%d", i)
		settleBatch(t, s, nudgeFailedCall(id))
		if i == 6 {
			break
		}
		notice, first, err := s.Take(context.Background(), []string{id})
		if err != nil {
			t.Fatal(err)
		}
		if i == 3 || i == 5 {
			if notice == nil || notice.Count != i || notice.CallID != id || !first || notice.Status != toolFailureStatusRecoverable {
				t.Fatalf("notice%d: %+v / %v", i, notice, first)
			}
		} else if notice != nil {
			t.Fatalf("unexpected notice%d: %+v", i, notice)
		}
	}
	if !errors.Is(s.terminalErr(), errLoopDetected) {
		t.Fatal("six identical calls did not stop")
	}
}

func TestNudgeStateSuccessCountsTowardStopOnly(t *testing.T) {
	s := newNudgeState()
	for i := 1; i <= 6; i++ {
		id := fmt.Sprintf("ok%d", i)
		settleBatch(t, s, nudgeOkCall(id))
		if i == 6 {
			break
		}
		if n, _, err := s.Take(context.Background(), []string{id}); n != nil || err != nil {
			t.Fatalf("success notice: %+v %v", n, err)
		}
	}
	if !errors.Is(s.terminalErr(), errLoopDetected) {
		t.Fatal("successful identical loop escaped existing guard")
	}
}

func TestNudgeStateCanonicalArgsIdentity(t *testing.T) {
	a, err := canonicalToolArguments([]byte(`{"x":9007199254740993,"y":1}`))
	if err != nil {
		t.Fatal(err)
	}
	b, err := canonicalToolArguments([]byte(`{ "y":1, "x":9007199254740993 }`))
	if err != nil || string(a) != string(b) {
		t.Fatalf("canonical arguments: %s / %s / %v", a, b, err)
	}
	s := newNudgeState()
	for i := 1; i <= 3; i++ {
		call := nudgeFailedCall(fmt.Sprint(i))
		call.ArgsJSON = string(a)
		settleBatch(t, s, call)
	}
	if n, _, err := s.Take(context.Background(), []string{"3"}); err != nil || n == nil || n.Count != 3 {
		t.Fatalf("canonical reminder: %+v %v", n, err)
	}
}

func TestNudgeStateTupleChanges(t *testing.T) {
	for _, field := range []string{"name", "arguments", "result", "error"} {
		t.Run(field, func(t *testing.T) {
			s := newNudgeState()
			for i := 0; i < 3; i++ {
				call := nudgeFailedCall(fmt.Sprint(i))
				value := fmt.Sprint(i)
				switch field {
				case "name":
					call.Name = value
				case "arguments":
					call.ArgsJSON = value
				case "result":
					call.Result = value
				case "error":
					call.Error = value
				}
				settleBatch(t, s, call)
			}
			if n, _, err := s.Take(context.Background(), []string{"2"}); err != nil || n != nil {
				t.Fatalf("changed tuple repeated: %+v %v", n, err)
			}
		})
	}
}

func TestNudgeStateEvaluatesNativeRequestOrder(t *testing.T) {
	s := newNudgeState()
	for _, key := range []string{"a", "b"} {
		for i := 1; i <= 4; i++ {
			c := nudgeFailedCall(fmt.Sprintf("%s%d", key, i))
			c.ArgsJSON = key
			settleBatch(t, s, c)
		}
	}
	a, b := nudgeFailedCall("a5"), nudgeFailedCall("b5")
	a.ArgsJSON = "a"
	b.ArgsJSON = "b"
	if err := s.record(b, false, true); err != nil {
		t.Fatal(err)
	}
	if err := s.record(a, false, true); err != nil {
		t.Fatal(err)
	}
	n, _, err := s.Take(context.Background(), []string{"a5", "b5"})
	if err != nil || n == nil || n.Count != 5 || n.CallID != "a5" {
		t.Fatalf("request-order tie: %+v %v", n, err)
	}
}

func TestNudgeStateRequiresAllProducerCoverageWithoutWaiting(t *testing.T) {
	s := newNudgeState()
	if err := s.record(nudgeFailedCall("a"), false, true); err != nil {
		t.Fatal(err)
	}
	if _, _, err := s.Take(context.Background(), []string{"a", "missing"}); err == nil {
		t.Fatal("missing producer accepted")
	}
	if len(s.window.calls) != 0 {
		t.Fatal("partial batch advanced detector before coverage validation")
	}
	if err := s.record(nudgeFailedCall("b"), false, true); err != nil {
		t.Fatal(err)
	}
	if _, _, err := s.Take(context.Background(), []string{"a", "b"}); err != nil {
		t.Fatal(err)
	}
	settleBatch(t, s, nudgeFailedCall("c"))
	if n, _, err := s.Take(context.Background(), []string{"c"}); err != nil || n == nil || n.Count != 3 {
		t.Fatalf("coverage counted partial inputs: %+v %v", n, err)
	}
}

func TestNudgeStateResumeExcludesCompletedSiblings(t *testing.T) {
	s := newNudgeState()
	settleBatch(t, s, nudgeFailedCall("one"))
	settleBatch(t, s, nudgeFailedCall("two"))
	if err := s.record(nudgeFailedCall("cached"), false, false); err != nil {
		t.Fatal(err)
	}
	if err := s.record(nudgeFailedCall("fresh"), false, true); err != nil {
		t.Fatal(err)
	}
	ids := []string{"cached", "fresh"}
	n, first, err := s.Take(context.Background(), ids)
	if err != nil || n == nil || n.CallID != "fresh" || n.Count != 3 || !first {
		t.Fatalf("cached sibling advanced fresh detector: %+v %v", n, err)
	}
	again, first, err := s.Take(context.Background(), ids)
	if err != nil || again != n || first {
		t.Fatal("native batch retry counted or scheduled twice")
	}
}

func TestNudgeStateInvariants(t *testing.T) {
	s := newNudgeState()
	if err := s.record(completedCall{}, false, true); err == nil {
		t.Fatal("empty producer id accepted")
	}
	call := nudgeFailedCall("same")
	if err := s.record(call, false, true); err != nil {
		t.Fatal(err)
	}
	changed := call
	changed.Result = "different"
	if err := s.record(changed, false, true); err == nil {
		t.Fatal("conflicting duplicate outcome accepted")
	}
	if _, _, err := s.Take(context.Background(), []string{"same", "same"}); err == nil {
		t.Fatal("duplicate native id accepted")
	}
	cause := errors.New("original persistence cause")
	s.Abort(cause)
	if _, _, err := s.Take(context.Background(), []string{"same"}); !errors.Is(err, cause) {
		t.Fatalf("abort lost cause: %v", err)
	}
}

func TestNudgeStateFreshResumeLeg(t *testing.T) {
	old := newNudgeState()
	for i := 0; i < 3; i++ {
		settleBatch(t, old, nudgeFailedCall(fmt.Sprint(i)))
	}
	fresh := newNudgeState()
	settleBatch(t, fresh, nudgeFailedCall("new-leg"))
	if n, _, err := fresh.Take(context.Background(), []string{"new-leg"}); n != nil || err != nil {
		t.Fatalf("stale detector state: %+v %v", n, err)
	}
}

func TestNudgeStateContextRoundTrip(t *testing.T) {
	s := newNudgeState()
	if nudgeStateFromContext(withNudgeState(context.Background(), s)) != s || nudgeStateFromContext(context.Background()) != nil {
		t.Fatal("leg context binding changed")
	}
}

// A journal append failure at the last result of a parallel batch must
// not strand a waiting boundary or leak a stale notice: the batch never
// seals, the run closes via the classified terminal, and stored events
// reflect only successful appends (ND-2).
func TestNudgeJournalFailure(t *testing.T) {
	ctx := context.Background()
	backend, err := sqlite.Open(ctx, filepath.Join(t.TempDir(), "nudge-jfail.db"))
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { _ = backend.Close() })

	sentinel := errors.New("journal write failed")
	journal := &contractJournal{
		inner: backend,

		failed: map[string]error{"tool.finished:call-b": sentinel},
	}

	ts, err := tools.Builtin(backend).Resolve([]string{tools.EchoInfoName})
	if err != nil {
		t.Fatalf("resolve tools: %v", err)
	}
	// One parallel batch: a single assistant turn requests two calls.
	script := []*schema.AgenticMessage{
		testToolCalls(testCall("call-a", tools.EchoInfoName, `{"text":"spin"}`), testCall("call-b", tools.EchoInfoName, `{"text":"spin"}`)),
		testAssistant("never reached"),
	}
	eng, err := NewEngine(ctx, NewScriptedModel(script...), ts, EngineConfig{
		StreamBuffer: 8, MaxEventPayloadBytes: 64 << 10, MaxToolTurns: 8,
	})
	if err != nil {
		t.Fatalf("new engine: %v", err)
	}
	sink := newTestSink()
	svc := NewService(eng, "scripted", "scripted-v0", ServiceDeps{
		Journal: journal, ToolOperations: backend, Runs: backend, Messages: backend, Notes: backend, Sink: sink,
	})

	runID, err := svc.Run(ctx, "sess-nudge-jfail", "echo twice")
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	waitForRunStatus(t, backend, runID, domain.RunFailed)

	events := replayAll(t, backend, runID)
	last := events[len(events)-1]
	if last.Type != domain.EventRunFailed {
		t.Fatalf("last event = %s, want run.failed", last.Type)
	}
	if n := countTerminal(events); n != 1 {
		t.Fatalf("terminal events = %d, want exactly 1", n)
	}
	// Only successful appends are stored: the failed call's tool.finished
	// is absent (its tool.started, appended in the same emit, may remain).
	finishedIDs := map[string]bool{}
	nudges := 0
	for _, ev := range events {
		switch ev.Type {
		case domain.EventToolFinished:
			var p struct {
				ToolCallID string `json:"tool_call_id"`
			}
			if err := json.Unmarshal(ev.Payload, &p); err == nil {
				finishedIDs[p.ToolCallID] = true
			}
		case domain.EventToolNudge:
			nudges++
		}
	}
	if finishedIDs["call-b"] {
		t.Fatal("tool.finished for the failed append reached the journal")
	}
	if nudges != 0 {
		t.Fatalf("tool.nudge events = %d, want 0 (no stale notice)", nudges)
	}
}
