package runtime

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/cloudwego/eino/schema"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/storage/sqlite"
	"agent-vivy/internal/tools"
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

// nudgeBatch runs one full registered batch through the state.
func settleBatch(t *testing.T, s *nudgeState, calls ...completedCall) {
	t.Helper()
	ids := make([]string, 0, len(calls))
	for _, c := range calls {
		ids = append(ids, c.ID)
	}
	if err := s.Register(ids); err != nil {
		t.Fatalf("register %v: %v", ids, err)
	}
	for _, c := range calls {
		if err := s.Complete(c); err != nil {
			t.Fatalf("complete %s: %v", c.ID, err)
		}
	}
	s.Seal(nil)
}

func TestNudgeStateRepetitionReminders(t *testing.T) {
	s := newNudgeState()
	ctx := context.Background()
	for i := 1; i <= 6; i++ {
		id := fmt.Sprintf("call-%d", i)
		settleBatch(t, s, nudgeFailedCall(id))
		if s.terminalErr() != nil {
			break
		}
		notice, _, err := s.Take(ctx, []string{id})
		if err != nil {
			t.Fatalf("take after batch %d: %v", i, err)
		}
		switch i {
		case 3, 5:
			if notice == nil {
				t.Fatalf("batch %d produced no notice", i)
			}
			if notice.Count != i || notice.CallID != id || notice.ToolName != "echo_info" {
				t.Fatalf("notice = %+v, want call %s count %d", notice, id, i)
			}
			if notice.Reason != "remote_tool_error" || notice.TemplateVersion != nudgeTemplateVersion {
				t.Fatalf("notice fields = %+v", notice)
			}
		default:
			if notice != nil {
				t.Fatalf("batch %d produced unexpected notice %+v", i, notice)
			}
		}
	}
	// The sixth identical call is the hard stop, not a reminder.
	if err := s.terminalErr(); !errors.Is(err, errLoopDetected) {
		t.Fatalf("terminal = %v, want errLoopDetected", err)
	}
	if _, _, err := s.Take(ctx, []string{"call-6"}); !errors.Is(err, errLoopDetected) {
		t.Fatalf("take after stop = %v, want errLoopDetected", err)
	}
}

// Successful identical calls never produce reminders but still count
// toward the hard stop at six (NUDGE-DESIGN §6).
func TestNudgeStateSuccessCountsTowardStopOnly(t *testing.T) {
	s := newNudgeState()
	ctx := context.Background()
	for i := 1; i <= 6; i++ {
		settleBatch(t, s, nudgeOkCall(fmt.Sprintf("call-%d", i)))
		if s.terminalErr() != nil {
			break
		}
		notice, _, err := s.Take(ctx, []string{fmt.Sprintf("call-%d", i)})
		if err != nil {
			t.Fatalf("take after batch %d: %v", i, err)
		}
		if notice != nil {
			t.Fatalf("successful batch %d produced notice %+v", i, notice)
		}
	}
	if err := s.terminalErr(); !errors.Is(err, errLoopDetected) {
		t.Fatalf("terminal = %v, want errLoopDetected", err)
	}
}

// The same call written with reordered JSON object keys shares one
// canonical identity: signature is taken from the canonical args the
// mapper captured at request time.
func TestNudgeStateCanonicalArgsIdentity(t *testing.T) {
	var a, b map[string]any
	if err := json.Unmarshal([]byte(`{"text":"spin","mode":"x"}`), &a); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal([]byte(`{"mode":"x","text":"spin"}`), &b); err != nil {
		t.Fatal(err)
	}
	argsA, _ := json.Marshal(a)
	argsB, _ := json.Marshal(b)
	if string(argsA) != string(argsB) {
		t.Fatalf("canonical args differ: %q vs %q", argsA, argsB)
	}

	s := newNudgeState()
	for i := 1; i <= 3; i++ {
		call := nudgeFailedCall(fmt.Sprintf("call-%d", i))
		call.ArgsJSON = string(argsA)
		settleBatch(t, s, call)
	}
	notice, _, err := s.Take(context.Background(), []string{"call-3"})
	if err != nil || notice == nil || notice.Count != 3 {
		t.Fatalf("notice = %+v, err = %v; want count-3 reminder", notice, err)
	}
}

// A changed result or changed argument is a different signature: the
// identical-call counter must not climb.
func TestNudgeStateSignatureChanges(t *testing.T) {
	s := newNudgeState()
	ctx := context.Background()
	results := []string{"r1", "r2", "r3"}
	for i, res := range results {
		call := nudgeFailedCall(fmt.Sprintf("res-%d", i))
		call.Result = res
		settleBatch(t, s, call)
	}
	if notice, _, err := s.Take(ctx, []string{"res-2"}); err != nil || notice != nil {
		t.Fatalf("changed results produced notice %+v err %v", notice, err)
	}
	args := []string{`{"text":"a"}`, `{"text":"b"}`, `{"text":"c"}`}
	for i, arg := range args {
		call := nudgeFailedCall(fmt.Sprintf("arg-%d", i))
		call.ArgsJSON = arg
		settleBatch(t, s, call)
	}
	if notice, _, err := s.Take(ctx, []string{"arg-2"}); err != nil || notice != nil {
		t.Fatalf("changed arguments produced notice %+v err %v", notice, err)
	}
}

func TestNudgeStateResumeBatchWaitsForEveryToolResult(t *testing.T) {
	s := newNudgeState()
	if err := s.Complete(nudgeOkCall("resume-a")); err != nil {
		t.Fatalf("complete first resumed result: %v", err)
	}

	type takeResult struct {
		notice *nudgeNotice
		err    error
	}
	result := make(chan takeResult, 1)
	go func() {
		notice, _, err := s.Take(context.Background(), []string{"resume-a", "resume-b"})
		result <- takeResult{notice: notice, err: err}
	}()
	select {
	case <-result:
		t.Fatal("model boundary released before every resumed tool result was durable")
	case <-time.After(20 * time.Millisecond):
	}

	if err := s.Complete(nudgeOkCall("resume-b")); err != nil {
		t.Fatalf("complete second resumed result: %v", err)
	}
	select {
	case got := <-result:
		if got.err != nil || got.notice != nil {
			t.Fatalf("take = %+v, want an unnudged sealed batch", got)
		}
	case <-time.After(time.Second):
		t.Fatal("model boundary remained blocked after all resumed tool results were durable")
	}
}

func TestNudgeStateResumeBatchAcceptsDurableSiblingResults(t *testing.T) {
	s := newNudgeState()
	if err := s.Register([]string{"resume-target", "resume-sibling"}); err != nil {
		t.Fatalf("register resumed batch: %v", err)
	}
	if err := s.SatisfyDurable("resume-sibling"); err != nil {
		t.Fatalf("satisfy durable sibling: %v", err)
	}
	if err := s.Complete(nudgeOkCall("resume-target")); err != nil {
		t.Fatalf("complete resumed target: %v", err)
	}
	notice, _, err := s.Take(context.Background(), []string{"resume-target", "resume-sibling"})
	if err != nil || notice != nil {
		t.Fatalf("take = %+v, want an unnudged batch with durable sibling; err=%v", notice, err)
	}
}

// Seal evaluates request order, not completion order: when two distinct
// failures in one batch tie on the threshold, the earlier request wins
// (ND-2: one notice per batch, ties earliest request index).
func TestNudgeStateSealEvaluatesRequestOrder(t *testing.T) {
	s := newNudgeState()
	ctx := context.Background()
	// Four prior identical failures per signature set up the tie at 5.
	for i := 1; i <= 4; i++ {
		call := nudgeFailedCall(fmt.Sprintf("a%d", i))
		call.ArgsJSON = `{"k":"a"}`
		settleBatch(t, s, call)
	}
	for i := 1; i <= 4; i++ {
		call := nudgeFailedCall(fmt.Sprintf("b%d", i))
		call.ArgsJSON = `{"k":"b"}`
		settleBatch(t, s, call)
	}
	a5 := nudgeFailedCall("a5")
	a5.ArgsJSON = `{"k":"a"}`
	b5 := nudgeFailedCall("b5")
	b5.ArgsJSON = `{"k":"b"}`
	if err := s.Register([]string{"a5", "b5"}); err != nil {
		t.Fatalf("register: %v", err)
	}
	// Completion order is the reverse of request order.
	if err := s.Complete(b5); err != nil {
		t.Fatalf("complete b5: %v", err)
	}
	if err := s.Complete(a5); err != nil {
		t.Fatalf("complete a5: %v", err)
	}
	s.Seal(nil)
	notice, _, err := s.Take(ctx, []string{"a5", "b5"})
	if err != nil {
		t.Fatalf("take: %v", err)
	}
	if notice == nil || notice.CallID != "a5" || notice.Count != 5 {
		t.Fatalf("notice = %+v, want a5 at count 5 (earliest request index)", notice)
	}
}

// Batch invariants fail the run: duplicated ids in a registration,
// a registration overlapping an unsealed batch, and results for ids
// the batch never requested (or already delivered).
func TestNudgeStateInvariants(t *testing.T) {
	t.Run("DuplicateRegistrationID", func(t *testing.T) {
		s := newNudgeState()
		if err := s.Register([]string{"x", "x"}); err == nil {
			t.Fatal("duplicate id registered without error")
		}
	})
	t.Run("EmptyBatch", func(t *testing.T) {
		s := newNudgeState()
		if err := s.Register(nil); err == nil {
			t.Fatal("empty batch registered without error")
		}
	})
	t.Run("OverlappingBatch", func(t *testing.T) {
		s := newNudgeState()
		if err := s.Register([]string{"a"}); err != nil {
			t.Fatalf("register: %v", err)
		}
		if err := s.Register([]string{"b"}); err == nil {
			t.Fatal("overlapping register did not error")
		}
	})
	t.Run("ForeignCompletion", func(t *testing.T) {
		s := newNudgeState()
		if err := s.Register([]string{"a", "b"}); err != nil {
			t.Fatalf("register: %v", err)
		}
		if err := s.Complete(nudgeFailedCall("z")); err == nil {
			t.Fatal("completion for unrequested id did not error")
		}
	})
	t.Run("DuplicateResultID", func(t *testing.T) {
		s := newNudgeState()
		if err := s.Register([]string{"a", "b"}); err != nil {
			t.Fatalf("register: %v", err)
		}
		if err := s.Complete(nudgeFailedCall("a")); err != nil {
			t.Fatalf("first complete: %v", err)
		}
		if err := s.Complete(nudgeFailedCall("a")); err == nil {
			t.Fatal("duplicate completion did not error")
		}
	})
	// A completion with no outstanding batch is admitted as the resume
	// leg's implicit singleton (the decided call replays without its
	// tool.requested).
	t.Run("ImplicitSingleton", func(t *testing.T) {
		s := newNudgeState()
		if err := s.Complete(nudgeFailedCall("solo")); err != nil {
			t.Fatalf("singleton completion: %v", err)
		}
		s.Seal(nil)
		if err := s.terminalErr(); err != nil {
			t.Fatalf("terminal = %v", err)
		}
	})
}

// Take is the boundary's only blocking API: it waits for the outstanding
// batch to seal, releases on cancellation and on Abort, and never hands
// out a partial batch.
func TestNudgeStateTakeWaitsForSeal(t *testing.T) {
	s := newNudgeState()
	if err := s.Register([]string{"a", "b"}); err != nil {
		t.Fatalf("register: %v", err)
	}
	done := make(chan error, 1)
	go func() {
		_, _, err := s.Take(context.Background(), []string{"a", "b"})
		done <- err
	}()
	select {
	case err := <-done:
		t.Fatalf("take returned %v before the batch sealed", err)
	case <-time.After(50 * time.Millisecond):
	}
	if err := s.Complete(nudgeFailedCall("a")); err != nil {
		t.Fatalf("complete a: %v", err)
	}
	select {
	case err := <-done:
		t.Fatalf("take returned %v while one result was still outstanding", err)
	case <-time.After(50 * time.Millisecond):
	}
	if err := s.Complete(nudgeFailedCall("b")); err != nil {
		t.Fatalf("complete b: %v", err)
	}
	s.Seal(nil)
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("take after seal: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("take still waiting after seal")
	}

	// Cancellation releases a wait without any seal.
	s2 := newNudgeState()
	if err := s2.Register([]string{"a"}); err != nil {
		t.Fatalf("register: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done2 := make(chan error, 1)
	go func() {
		_, _, err := s2.Take(ctx, []string{"a"})
		done2 <- err
	}()
	cancel()
	select {
	case err := <-done2:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("cancelled take = %v, want context.Canceled", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("cancel did not release take")
	}

	// Abort releases every waiter with the given cause.
	s3 := newNudgeState()
	if err := s3.Register([]string{"a"}); err != nil {
		t.Fatalf("register: %v", err)
	}
	done3 := make(chan error, 1)
	go func() {
		_, _, err := s3.Take(context.Background(), []string{"a"})
		done3 <- err
	}()
	cause := errors.New("journal write failed")
	s3.Abort(cause)
	select {
	case err := <-done3:
		if !errors.Is(err, cause) {
			t.Fatalf("aborted take = %v, want %v", err, cause)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("abort did not release take")
	}
	if _, _, err := s3.Take(context.Background(), []string{"a"}); !errors.Is(err, cause) {
		t.Fatalf("take after abort = %v, want %v", err, cause)
	}
}

// One scheduling signal per sealed batch: a second Take sees the same
// notice (a provider retry must observe the identical request) but
// reports first=false, so the journal emitter fires only once.
func TestNudgeStateTakeOncePerBatch(t *testing.T) {
	s := newNudgeState()
	for i := 1; i <= 3; i++ {
		settleBatch(t, s, nudgeFailedCall(fmt.Sprintf("call-%d", i)))
	}
	ctx := context.Background()
	first, firstHandout, err := s.Take(ctx, []string{"call-3"})
	if err != nil || first == nil || !firstHandout {
		t.Fatalf("first take = %+v first=%v err=%v; want the count-3 notice scheduled", first, firstHandout, err)
	}
	second, secondHandout, err := s.Take(ctx, []string{"call-3"})
	if err != nil {
		t.Fatalf("second take: %v", err)
	}
	if second == nil || second.CallID != first.CallID || secondHandout {
		t.Fatalf("second take = %+v first=%v; want the same notice without a new scheduling signal", second, secondHandout)
	}
}

// A resume leg starts with an empty window and no pending notice: a
// fresh state answers a barrier-free Take (no trailing results)
// immediately.
func TestNudgeStateFreshResumeLeg(t *testing.T) {
	s := newNudgeState()
	notice, _, err := s.Take(context.Background(), nil)
	if err != nil || notice != nil {
		t.Fatalf("fresh state take = %+v, %v; want nil notice", notice, err)
	}
}

// The context seam shares exactly one detector instance per leg.
func TestNudgeStateContextRoundTrip(t *testing.T) {
	if got := nudgeStateFromContext(context.Background()); got != nil {
		t.Fatalf("nudgeStateFromContext = %v, want nil", got)
	}
	s := newNudgeState()
	ctx := withNudgeState(context.Background(), s)
	if got := nudgeStateFromContext(ctx); got != s {
		t.Fatalf("nudgeStateFromContext = %p, want %p", got, s)
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
		inner:  backend,
		holder: &contractStateHolder{},
		failed: map[string]error{"tool.finished:call-b": sentinel},
	}

	ts, err := tools.Builtin(backend).Resolve([]string{tools.EchoInfoName})
	if err != nil {
		t.Fatalf("resolve tools: %v", err)
	}
	// One parallel batch: a single assistant turn requests two calls.
	script := []*schema.Message{
		schema.AssistantMessage("", []schema.ToolCall{
			{ID: "call-a", Function: schema.FunctionCall{Name: tools.EchoInfoName, Arguments: `{"text":"spin"}`}},
			{ID: "call-b", Function: schema.FunctionCall{Name: tools.EchoInfoName, Arguments: `{"text":"spin"}`}},
		}),
		schema.AssistantMessage("never reached", nil),
	}
	eng, err := NewEngine(ctx, NewScriptedModel(script...), ts, EngineConfig{
		StreamBuffer: 8, MaxEventPayloadBytes: 64 << 10, MaxToolTurns: 8,
	})
	if err != nil {
		t.Fatalf("new engine: %v", err)
	}
	sink := newTestSink()
	svc := NewService(eng, "scripted", "scripted-v0", ServiceDeps{
		Journal: journal, Runs: backend, Messages: backend, Notes: backend, Sink: sink,
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
