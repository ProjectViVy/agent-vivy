package runtime

import (
	"agent-vivy/internal/domain"
	"agent-vivy/internal/tools"
	"context"
	"errors"
	"fmt"
	"github.com/cloudwego/eino/schema"
	"strings"
	"testing"
	"time"
)

func TestLoopWindowExactTupleDoesNotAliasFields(t *testing.T) {
	var window loopWindow
	if count, err := window.record("a", "b\x00c", "", ""); err != nil || count != 1 {
		t.Fatalf("first tuple: %d %v", count, err)
	}
	if count, err := window.record("a\x00b", "c", "", ""); err != nil || count != 1 {
		t.Fatalf("distinct tuple fields alias: %d %v", count, err)
	}
}

// The derived mapper consumer may lag; the native producer must already have
// committed and exposed its typed outcome before the next WrapModel entry.
func TestNudgeDoesNotWaitForEventConsumer(t *testing.T) {
	h := newContractHarness(t, []*schema.AgenticMessage{
		testToolCalls(contractToolCall("native-third", "x")), testAssistant("done"),
	}, contractHarnessOpts{streaming: true, prodNudge: true, extraTools: []tools.Tool{failingContractTool()}})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	const run domain.RunID = "run-native-consumer"
	const session domain.SessionID = "session-native-consumer"
	mustCreateSession(t, h.backend, session)
	if err := h.backend.CreateRun(ctx, domain.Run{ID: run, SessionID: session, Status: domain.RunActive, CreatedAt: 1}); err != nil {
		t.Fatal(err)
	}
	state := newNudgeState()
	failure, ok := classifyToolFailure(ctx, failingContractTool().Spec(), &tools.ArgError{Field: "text", Reason: "must be a string"})
	if !ok {
		t.Fatal("missing typed failure")
	}
	for _, id := range []string{"prime1", "prime2"} {
		settleBatch(t, state, completedCall{ID: id, Name: contractToolName, ArgsJSON: `{"text":"x"}`, Result: untrustedToolResultHeader + failure.Diagnostic, Error: failure.Diagnostic, Failure: &failure})
	}
	ctx = withNudgeState(withSessionID(withRunID(ctx, run), session), state)
	ctx = withToolOperationCoordinator(ctx, h.svc.newToolOperationCoordinator(run, session))
	mapper := newEventMapper(run, 64<<10)
	mapper.setNudgeState(state)
	h.svc.bindMapperToolOutcomes(ctx, mapper)
	ctx = withNudgeEmitter(ctx, h.svc.nudgeEmitter(mapper, session))
	it := h.svc.engine.Query(ctx, "repeat the failed call")
	paused := make(chan struct{})
	release := make(chan struct{})
	done := make(chan error, 1)
	go func() {
		for {
			ev, ok := it.Next()
			if !ok {
				done <- nil
				return
			}
			if err := mapper.onEventEach(ev, func(events []domain.RunEvent) error {
				for _, event := range events {
					if event.Type == domain.EventToolFinished {
						close(paused)
						select {
						case <-release:
						case <-ctx.Done():
							return ctx.Err()
						}
					}
				}
				return nil
			}); err != nil {
				done <- err
				return
			}
		}
	}()
	released := false
	defer func() {
		if !released {
			close(release)
		}
		cancel()
		select {
		case <-done:
		case <-time.After(time.Second):
			t.Error("native consumer did not exit")
		}
	}()
	select {
	case <-paused:
	case err := <-done:
		t.Fatalf("consumer ended before result: %v", err)
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	deadline := time.Now().Add(2 * time.Second)
	for h.model.count() < 2 && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if h.model.count() != 2 {
		t.Fatal("next native model request waits for derived consumer acknowledgement")
	}
	if notices := nudgeMessages(h.model.entry(1).input); len(notices) != 1 {
		t.Fatalf("eligible reminder missing: %d", len(notices))
	}
	op, err := h.backend.GetToolOperation(ctx, run, "native-third")
	if err != nil || op.State != domain.ToolOperationCompleted {
		t.Fatalf("producer did not commit: %+v %v", op, err)
	}
	close(release)
	released = true
	if got := nudgeEvents(t, h.journal); len(got) != 1 {
		t.Fatalf("scheduling count: %d", len(got))
	}
}

func TestNudgeSchedulingFailureRetainsCauseAcrossRetry(t *testing.T) {
	state := newNudgeState()
	for _, id := range []string{"a", "b", "c"} {
		settleBatch(t, state, nudgeFailedCall(id))
	}
	sentinel := errors.New("scheduling append failed")
	ctx := withNudgeState(context.Background(), state)
	attempts := 0
	ctx = withNudgeEmitter(ctx, func(context.Context, nudgeNotice) error { attempts++; return sentinel })
	input := []*schema.AgenticMessage{agenticToolResultMessage("c", "echo_info", "failed")}
	wrapped := &nudgeWrappedModel{inner: &recordingInner{}}
	for i := 0; i < 2; i++ {
		if _, err := wrapped.prepare(ctx, input); !errors.Is(err, sentinel) {
			t.Fatalf("retry%d lost cause: %v", i, err)
		}
	}
	if attempts != 1 {
		t.Fatalf("scheduling attempts: %d", attempts)
	}
}

func TestNudgeDistinctMalformedArgumentsRemainDistinct(t *testing.T) {
	var script []*schema.AgenticMessage
	for i := 0; i < 6; i++ {
		script = append(script, testToolCalls(testCall(fmt.Sprint("invalid", i), contractToolName, fmt.Sprintf("{bad-%d", i))))
	}
	script = append(script, testAssistant("handled distinct invalid inputs"))
	h := newContractHarness(t, script, contractHarnessOpts{streaming: true, prodNudge: true})
	run, err := h.svc.Run(context.Background(), "invalid-args-session", "handle malformed calls")
	if err != nil {
		t.Fatal(err)
	}
	waitForTerminalRun(t, h.backend, run)
	stored, err := h.backend.GetRun(context.Background(), run)
	if err != nil || stored.Status != domain.RunCompleted {
		t.Fatalf("distinct malformed inputs triggered a loop: %+v %v", stored, err)
	}
	if len(nudgeEvents(t, h.journal)) != 0 {
		t.Fatal("distinct invalid inputs received identical-call coaching")
	}
}

func TestNudgeProjectionRejectsOversizedValues(t *testing.T) {
	large := strings.Repeat("x", (1<<20)+1)
	for _, tc := range []struct {
		args    string
		outcome domain.ToolOperationOutcome
	}{
		{large, successToolOutcome("ok")}, {`{}`, successToolOutcome(large)}, {`{}`, domain.ToolOperationOutcome{Failure: large}},
	} {
		ctx := withNudgeState(context.Background(), newNudgeState())
		if err := recordNudgeProjection(ctx, officialToolSearchName, tc.args, tc.outcome); !errors.Is(err, ErrContextBudgetExceeded) {
			t.Fatalf("oversized producer value did not fail bounded: %v", err)
		}
	}
}

func TestNudgeSchedulingFailureHasOneTerminalOwner(t *testing.T) {
	h := newContractHarness(t, nil, contractHarnessOpts{})
	const run domain.RunID = "nudge-scheduling-owner"
	const session domain.SessionID = "nudge-scheduling-session"
	ctx := context.Background()
	mustCreateSession(t, h.backend, session)
	if err := h.backend.CreateRun(ctx, domain.Run{ID: run, SessionID: session, Status: domain.RunActive, CreatedAt: 1}); err != nil {
		t.Fatal(err)
	}
	sentinel := errors.New("scheduling disk failure")
	h.setFail("tool.nudge:third", sentinel)
	mapper := newEventMapper(run, 64<<10)
	err := h.svc.nudgeEmitter(mapper, session)(ctx, nudgeNotice{CallID: "third", ToolName: contractToolName, Reason: toolFailureReasonInvalidArguments, Count: 3, TemplateVersion: nudgeTemplateVersion})
	if !errors.Is(err, sentinel) {
		t.Fatalf("lost Journal cause: %v", err)
	}
	stored, getErr := h.backend.GetRun(ctx, run)
	if getErr != nil || stored.Status != domain.RunActive {
		t.Fatalf("scheduling emitter took terminal ownership: %+v %v", stored, getErr)
	}
	h.svc.emitTerminal(ctx, mapper, h.svc.terminalEvent(ctx, mapper, err))
	if events := replayAll(t, h.backend, run); countTerminal(events) != 1 {
		t.Fatalf("terminal count: %d", countTerminal(events))
	}
}
