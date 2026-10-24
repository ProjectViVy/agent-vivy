package runtime

// VCP-B1 follow-up: guidance survives a human interrupt, and "send now"
// interrupts the active run through the same durable queue admission path.

import (
	"context"
	"encoding/json"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/storage"
	"agent-vivy/internal/testsupport"
	"agent-vivy/internal/tools"
)

// gatedApprovalModel blocks its first call, then asks for the write_note
// approval; every later call answers plainly and records its input.
type gatedApprovalModel struct {
	entered chan struct{}
	release chan struct{}

	mu     sync.Mutex
	inputs [][]*schema.AgenticMessage
}

func newGatedApprovalModel() *gatedApprovalModel {
	return &gatedApprovalModel{entered: make(chan struct{}), release: make(chan struct{})}
}

func (m *gatedApprovalModel) next(ctx context.Context, input []*schema.AgenticMessage) (*schema.AgenticMessage, error) {
	m.mu.Lock()
	m.inputs = append(m.inputs, input)
	call := len(m.inputs)
	m.mu.Unlock()
	if call == 1 {
		close(m.entered)
		select {
		case <-m.release:
		case <-ctx.Done():
			return nil, ctx.Err()
		}
		return toolCallMessage(ApprovalFlowCallID, tools.WriteNoteName, `{"content":"buy milk"}`), nil
	}
	return testAssistant("done: handled"), nil
}

func (m *gatedApprovalModel) Generate(ctx context.Context, input []*schema.AgenticMessage, _ ...model.Option) (*schema.AgenticMessage, error) {
	return m.next(ctx, input)
}

func (m *gatedApprovalModel) Stream(ctx context.Context, input []*schema.AgenticMessage, _ ...model.Option) (*schema.StreamReader[*schema.AgenticMessage], error) {
	msg, err := m.next(ctx, input)
	if err != nil {
		return nil, err
	}
	return oneShot(msg), nil
}

// calledWith reports whether any model input carried text in a user message.
func (m *gatedApprovalModel) calledWith(text string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, input := range m.inputs {
		for _, msg := range input {
			if msg != nil && msg.Role == schema.AgenticRoleTypeUser && strings.Contains(agenticMessageText(msg), text) {
				return true
			}
		}
	}
	return false
}

func TestSteerSurvivesApprovalInterrupt(t *testing.T) {
	chat := newGatedApprovalModel()
	svc, backend, _ := newApprovalServiceWithModel(t, 5*time.Minute, chat)
	ctx := context.Background()
	const sessionID = domain.SessionID("sess-steer-approval")
	mustCreateSession(t, backend, sessionID)

	runID, err := svc.Run(ctx, sessionID, "initial prompt")
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	<-chat.entered

	item, err := svc.Steer(ctx, sessionID, "steer survives approval")
	if err != nil {
		t.Fatalf("steer: %v", err)
	}
	if item.Track != domain.QueueTrackSteer {
		t.Fatalf("track = %q, want steer", item.Track)
	}
	close(chat.release)

	approval := waitForPendingApproval(t, backend, runID)
	// The crux: the approval suspension must not consume the steer lane.
	state := svc.QueueState(ctx, sessionID, "")
	if len(state.Steering) != 1 || state.Steering[0].Text != "steer survives approval" {
		t.Fatalf("steer lane after approval = %+v, want the queued guidance", state.Steering)
	}

	if err := svc.DecideApproval(ctx, approval.ID, domain.ApprovalApproved); err != nil {
		t.Fatalf("decide: %v", err)
	}
	waitForRunStatus(t, backend, runID, domain.RunCompleted)

	if !chat.calledWith("steer survives approval") {
		t.Fatal("guidance never reached the model after the approval resumed")
	}
	steered := 0
	for _, ev := range journalEvents(t, backend, runID) {
		if ev.Type == domain.EventTurnSteered {
			steered++
		}
	}
	if steered != 1 {
		t.Fatalf("turn.steered count = %d, want 1", steered)
	}
	after := svc.QueueState(ctx, sessionID, "")
	if len(after.Steering) != 0 || len(after.FollowUps) != 0 {
		t.Fatalf("queue after delivery = %+v", after)
	}
}

func TestSteerCancelStillFlushes(t *testing.T) {
	chat := newGatedApprovalModel()
	svc, backend, _ := newApprovalServiceWithModel(t, 5*time.Minute, chat)
	ctx := context.Background()
	const sessionID = domain.SessionID("sess-steer-cancel")
	mustCreateSession(t, backend, sessionID)

	runID, err := svc.Run(ctx, sessionID, "initial prompt")
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	<-chat.entered
	item, err := svc.Steer(ctx, sessionID, "steer lost on cancel")
	if err != nil {
		t.Fatalf("steer: %v", err)
	}
	if !svc.Cancel(runID) {
		t.Fatal("cancel reported no active run")
	}
	waitForRunStatus(t, backend, runID, domain.RunCancelled)

	for _, ev := range journalEvents(t, backend, runID) {
		if ev.Type == domain.EventTurnSteered {
			t.Fatal("a user cancel must not deliver the steer")
		}
	}
	// The flush tombstone rides the Session control stream: the run journal
	// is sealed by the terminal before the removal is persisted.
	aborted := false
	for _, ev := range journalEvents(t, backend, queueControlRunID(sessionID)) {
		if ev.Type != domain.EventTurnDequeued {
			continue
		}
		var p payloadTurnDequeued
		if json.Unmarshal(ev.Payload, &p) != nil {
			t.Fatalf("decode turn.dequeued: %s", ev.Payload)
		}
		if p.QueueID == item.ID && p.Reason == "aborted" && p.Text == "steer lost on cancel" {
			aborted = true
		}
	}
	if !aborted {
		t.Fatal("cancelled run did not journal an aborted turn.dequeued for the queued steer")
	}
	state := svc.QueueState(ctx, sessionID, "")
	if len(state.Steering) != 0 || len(state.FollowUps) != 0 {
		t.Fatalf("queue after cancel = %+v", state)
	}
}

func TestSendNowInterruptsCurrentRun(t *testing.T) {
	stm := newScriptedToolModel()
	svc, backend := newScriptedService(t, stm)
	ctx := context.Background()
	const sessionID = domain.SessionID("sess-sendnow")
	mustCreateSession(t, backend, sessionID)

	runID, err := svc.Run(ctx, sessionID, "first prompt")
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	<-stm.entered

	item, err := svc.SendNow(ctx, sessionID, "send now text")
	if err != nil {
		t.Fatalf("send now: %v", err)
	}
	if item.Track != domain.QueueTrackFollowUp || !item.InterruptCurrent {
		t.Fatalf("send-now item = %+v, want a flagged follow_up", item)
	}

	waitForRunStatus(t, backend, runID, domain.RunCancelled)
	waitFor(t, "send-now run admitted", func() bool {
		runs, err := backend.ListRunsBySession(ctx, sessionID)
		return err == nil && len(runs) == 2
	})
	runs, err := backend.ListRunsBySession(ctx, sessionID)
	if err != nil {
		t.Fatalf("list runs: %v", err)
	}
	nextRun := runs[1].ID
	waitForRunStatus(t, backend, nextRun, domain.RunCompleted)

	found := false
	for _, text := range stm.lastUserTexts() {
		if text == "send now text" {
			found = true
		}
	}
	if !found {
		t.Fatalf("send-now text absent from the admitted run's prompt: %v", stm.lastUserTexts())
	}
	queuedFlagged, cancelledAfterQueued := false, false
	for _, ev := range journalEvents(t, backend, runID) {
		switch ev.Type {
		case domain.EventTurnQueued:
			var p payloadTurnQueued
			if json.Unmarshal(ev.Payload, &p) != nil {
				t.Fatalf("decode turn.queued: %s", ev.Payload)
			}
			if p.QueueID == item.ID && p.InterruptCurrent {
				queuedFlagged = true
			}
		case domain.EventRunCancelled:
			cancelledAfterQueued = queuedFlagged
		}
	}
	if !queuedFlagged || !cancelledAfterQueued {
		t.Fatalf("journal order: flagged turn.queued=%v before cancel=%v", queuedFlagged, cancelledAfterQueued)
	}
	for _, ev := range journalEvents(t, backend, nextRun) {
		if ev.Type != domain.EventRunStarted {
			continue
		}
		var p payloadRunStarted
		if json.Unmarshal(ev.Payload, &p) != nil {
			t.Fatalf("decode run.started: %s", ev.Payload)
		}
		if p.Queue != nil {
			for _, pending := range p.Queue.Pending {
				if pending.InterruptCurrent {
					t.Fatal("admission left a send-now flag in the queue snapshot")
				}
			}
		}
	}
	state := svc.QueueState(ctx, sessionID, "")
	if len(state.Steering) != 0 || len(state.FollowUps) != 0 {
		t.Fatalf("queue after admission = %+v", state)
	}
}

// A send-now while the run is parked on an approval cancels the suspension
// and starts the next run from the flagged turn.
func TestSendNowCancelsSuspendedRun(t *testing.T) {
	chat := newGatedApprovalModel()
	svc, backend, _ := newApprovalServiceWithModel(t, 5*time.Minute, chat)
	ctx := context.Background()
	const sessionID = domain.SessionID("sess-sendnow-suspended")
	mustCreateSession(t, backend, sessionID)

	runID, err := svc.Run(ctx, sessionID, "initial prompt")
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	<-chat.entered
	close(chat.release)
	waitForPendingApproval(t, backend, runID)

	item, err := svc.SendNow(ctx, sessionID, "send now over approval")
	if err != nil {
		t.Fatalf("send now: %v", err)
	}
	if !item.InterruptCurrent {
		t.Fatalf("send-now item = %+v, want the interrupt flag", item)
	}
	waitForRunStatus(t, backend, runID, domain.RunCancelled)
	waitFor(t, "send-now run admitted", func() bool {
		runs, err := backend.ListRunsBySession(ctx, sessionID)
		return err == nil && len(runs) == 2
	})
	runs, err := backend.ListRunsBySession(ctx, sessionID)
	if err != nil {
		t.Fatalf("list runs: %v", err)
	}
	nextRun := runs[1].ID
	waitForRunStatus(t, backend, nextRun, domain.RunCompleted)
	if !chat.calledWith("send now over approval") {
		t.Fatal("send-now text never reached the admitted run")
	}
	if state := svc.QueueState(ctx, sessionID, ""); len(state.Steering) != 0 || len(state.FollowUps) != 0 {
		t.Fatalf("queue after suspended send-now = %+v", state)
	}
}

func TestSendNowSurvivesReplay(t *testing.T) {
	svc, backend := newQueueTestService(t, testsupport.NewEchoModel())
	ctx := context.Background()
	const sessionID = domain.SessionID("sess-sendnow-replay")
	mustCreateSession(t, backend, sessionID)
	runID := domain.RunID("run-sendnow-replay")
	if err := backend.CreateRun(ctx, domain.Run{ID: runID, SessionID: sessionID, Status: domain.RunCancelled, CreatedAt: 1}); err != nil {
		t.Fatalf("create run: %v", err)
	}
	m := newEventMapper(runID, 64<<10)
	events := []domain.RunEvent{
		m.build(domain.EventTurnQueued, payloadTurnQueued{QueueID: "q-keep", Track: domain.QueueTrackFollowUp, Text: "send now text", InterruptCurrent: true}),
		m.build(domain.EventTurnQueued, payloadTurnQueued{QueueID: "q-drop", Track: domain.QueueTrackFollowUp, Text: "ordinary follow-up"}),
		m.build(domain.EventRunCancelled, payloadRunCancelled{Reason: reasonUserRequested}),
	}
	if _, err := backend.Append(ctx, storage.Commit{RunID: runID, Events: events}); err != nil {
		t.Fatalf("append journal: %v", err)
	}

	state := svc.QueueState(ctx, sessionID, "")
	if state.Err != nil {
		t.Fatalf("queue replay: %v", state.Err)
	}
	if len(state.FollowUps) != 1 || state.FollowUps[0].ID != "q-keep" || !state.FollowUps[0].InterruptCurrent {
		t.Fatalf("replayed queue = %+v, want only the flagged send-now", state.FollowUps)
	}
}
