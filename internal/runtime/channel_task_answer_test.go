package runtime

import (
	"context"
	"errors"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/storage"
	"agent-vivy/internal/storage/sqlite"
	"agent-vivy/internal/tools"
)

// newChannelQuestionService wires a question-capable service with the full
// channel-task boundary: an A2A-admitted run that suspends on ask_user can
// be answered remotely through the same shared transition the local Review
// path uses.
func newChannelQuestionService(t *testing.T) (*Service, *sqlite.Backend, *testSink) {
	t.Helper()
	ctx := context.Background()
	backend, err := sqlite.Open(ctx, filepath.Join(t.TempDir(), "channel-questions.db"))
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { _ = backend.Close() })
	ts, err := tools.Builtin(backend).Resolve([]string{tools.AskUserName})
	if err != nil {
		t.Fatalf("resolve ask_user: %v", err)
	}
	checkpoints, err := NewVersionedCheckpointStore(backend.Blobs(), "test-engine")
	if err != nil {
		t.Fatalf("checkpoint store: %v", err)
	}
	eng, err := NewEngine(ctx, NewQuestionFlowModel(), ts, EngineConfig{
		StreamBuffer: 8, MaxEventPayloadBytes: 64 << 10, Checkpoints: checkpoints,
	})
	if err != nil {
		t.Fatalf("new engine: %v", err)
	}
	sink := newTestSink()
	svc := NewService(eng, "scripted", "scripted-v0", ServiceDeps{
		Journal: backend, Runs: backend, Messages: backend, Questions: backend,
		PrimaryRuns: backend, Admission: backend,
		ApprovalExpiration: 5 * time.Minute, Sink: sink,
	})
	svc.deps.GenerationID = "test-generation"
	return svc, backend, sink
}

func channelScope() domain.ChannelTaskScope {
	return domain.ChannelTaskScope{InstanceKey: "inst/a2a:a2a", PrincipalID: "p-1"}
}

// admitQuestionRun submits one channel message that launches a run which
// suspends on ask_user; returns the receipt and the pending question.
func admitQuestionRun(t *testing.T, svc *Service, backend *sqlite.Backend, messageID string) (domain.ChannelTaskReceipt, domain.Question) {
	t.Helper()
	receipt, err := svc.SubmitChannelTask(context.Background(), domain.ChannelTaskInput{
		Scope: channelScope(), MessageID: messageID, Parts: []string{"ask me"},
	})
	if err != nil {
		t.Fatalf("submit: %v", err)
	}
	question := waitForPendingQuestion(t, backend, receipt.RunID)
	return receipt, question
}

func replayChannelEvents(t *testing.T, backend *sqlite.Backend, runID domain.RunID) []domain.RunEvent {
	t.Helper()
	iter, err := backend.Replay(context.Background(), runID, 0)
	if err != nil {
		t.Fatalf("replay: %v", err)
	}
	defer func() { _ = iter.Close() }()
	var events []domain.RunEvent
	for iter.Next() {
		events = append(events, iter.Value().Event)
	}
	if err := iter.Err(); err != nil {
		t.Fatalf("replay iter: %v", err)
	}
	return events
}

func countEvents(t *testing.T, backend *sqlite.Backend, runID domain.RunID, eventType domain.EventType) int {
	t.Helper()
	n := 0
	for _, ev := range replayChannelEvents(t, backend, runID) {
		if ev.Type == eventType {
			n++
		}
	}
	return n
}

// TestChannelTaskAnswerDispatchAndAtomicity is the A2A-03.2 happy path plus
// the duplicate-suppression evidence: the remote ordinary answer commits
// question CAS + answered envelope + receipt atomically, schedules exactly
// one resume, and the identical retry resolves the original receipt.
func TestChannelTaskAnswerDispatchAndAtomicity(t *testing.T) {
	svc, backend, _ := newChannelQuestionService(t)
	ctx := context.Background()
	t.Cleanup(func() { svc.CancelAll(); svc.WaitIdle(ctx) })

	receipt, question := admitQuestionRun(t, svc, backend, "remote-q1")
	answerIn := domain.ChannelTaskInput{
		Scope: channelScope(), MessageID: "remote-ans-1",
		SessionID: receipt.SessionID, RunID: receipt.RunID,
		QuestionID: question.ID, Parts: []string{"  blue ", "sky"},
	}
	answerReceipt, err := svc.SubmitChannelTask(ctx, answerIn)
	if err != nil {
		t.Fatalf("answer: %v", err)
	}
	if answerReceipt.Operation != "answer" || answerReceipt.QuestionID != question.ID || answerReceipt.RunID != receipt.RunID {
		t.Fatalf("answer receipt = %+v", answerReceipt)
	}
	waitForRunStatus(t, backend, receipt.RunID, domain.RunCompleted)

	answered, err := backend.GetQuestion(ctx, question.ID)
	if err != nil || answered.Status != domain.QuestionAnswered || answered.Answer != "blue \n\nsky" {
		t.Fatalf("answered = %+v err=%v", answered, err)
	}
	if answered.Actor != "channel:a2a:p-1" {
		t.Fatalf("actor = %q", answered.Actor)
	}
	// One answered envelope, payload version 2 with the channel actor.
	if n := countEvents(t, backend, receipt.RunID, domain.EventUserQuestionAnswered); n != 1 {
		t.Fatalf("answered events = %d", n)
	}
	events := replayChannelEvents(t, backend, receipt.RunID)
	var sawV2 bool
	for _, ev := range events {
		if ev.Type == domain.EventUserQuestionAnswered {
			if ev.PayloadVersion != 2 {
				t.Fatalf("answered payload version = %d", ev.PayloadVersion)
			}
			sawV2 = true
		}
	}
	if !sawV2 {
		t.Fatal("no answered event journaled")
	}

	// Identical retry: original receipt, no second resume, no second event.
	retry, err := svc.SubmitChannelTask(ctx, answerIn)
	if err != nil {
		t.Fatalf("retry: %v", err)
	}
	if retry.AcceptedSeq != answerReceipt.AcceptedSeq {
		t.Fatalf("retry accepted_seq = %d, want %d", retry.AcceptedSeq, answerReceipt.AcceptedSeq)
	}
	if n := countEvents(t, backend, receipt.RunID, domain.EventUserQuestionAnswered); n != 1 {
		t.Fatalf("answered events after retry = %d", n)
	}
	if approvals, err := backend.ListPendingApprovals(ctx); err != nil || len(approvals) != 0 {
		t.Fatalf("approvals touched: %+v err=%v", approvals, err)
	}
}

// TestChannelTaskAnswerSelectorsAndLosers covers selector mismatch,
// settled-question conflict and guessed-id non-disclosure at the runtime
// boundary.
func TestChannelTaskAnswerSelectorsAndLosers(t *testing.T) {
	svc, backend, _ := newChannelQuestionService(t)
	ctx := context.Background()
	t.Cleanup(func() { svc.CancelAll(); svc.WaitIdle(ctx) })

	receipt, question := admitQuestionRun(t, svc, backend, "remote-q2")

	// Run selector mismatch: not-found, no disclosure, no receipt.
	if _, err := svc.SubmitChannelTask(ctx, domain.ChannelTaskInput{
		Scope: channelScope(), MessageID: "remote-ans-mm",
		SessionID: receipt.SessionID, RunID: "run_wrong",
		QuestionID: question.ID, Parts: []string{"blue"},
	}); !errors.Is(err, storage.ErrNotFound) {
		t.Fatalf("mismatch err = %v, want ErrNotFound", err)
	}
	if _, found, _ := backend.FindChannelTaskReceipt(ctx, channelScope(), "remote-ans-mm"); found {
		t.Fatal("mismatch left a receipt")
	}
	// Guessed question id on another scope's run: not-found either.
	if _, err := svc.SubmitChannelTask(ctx, domain.ChannelTaskInput{
		Scope: channelScope(), MessageID: "remote-ans-ghost",
		QuestionID: "que_does_not_exist", Parts: []string{"blue"},
	}); !errors.Is(err, storage.ErrNotFound) {
		t.Fatalf("ghost err = %v, want ErrNotFound", err)
	}

	// Winning answer commits; a losing message naming the settled
	// question can never settle it again.
	win, err := svc.SubmitChannelTask(ctx, domain.ChannelTaskInput{
		Scope: channelScope(), MessageID: "remote-ans-win",
		QuestionID: question.ID, Parts: []string{"blue"},
	})
	if err != nil || win.Operation != "answer" {
		t.Fatalf("win = %+v err=%v", win, err)
	}
	if _, err := svc.SubmitChannelTask(ctx, domain.ChannelTaskInput{
		Scope: channelScope(), MessageID: "remote-ans-loser",
		QuestionID: question.ID, Parts: []string{"green"},
	}); err == nil {
		t.Fatal("loser answered a settled question")
	}
	if _, found, _ := backend.FindChannelTaskReceipt(ctx, channelScope(), "remote-ans-loser"); found {
		t.Fatal("loser left a receipt")
	}
}

// TestA2AApprovalIsLocalOnly proves remote answer text — including
// "/approve", "/deny" and "/pending" lookalikes — only settles the
// question it names and can never enter command parsing or approve a
// tool. An approval id used as the question selector is plain not-found.
func TestA2AApprovalIsLocalOnly(t *testing.T) {
	svc, backend, _ := newChannelQuestionService(t)
	ctx := context.Background()
	t.Cleanup(func() { svc.CancelAll(); svc.WaitIdle(ctx) })

	receipt, question := admitQuestionRun(t, svc, backend, "remote-q3")

	// Pending approval on the same run: the remote answer must not touch it.
	if err := backend.CreateApproval(ctx, domain.Approval{
		ID: "apr_guard", RunID: receipt.RunID, ToolCallID: "call-approval",
		ToolName: "protected_tool", Decision: domain.ApprovalPending,
		CreatedAt: time.Now().UnixMilli(), ExpiresAt: time.Now().Add(time.Hour).UnixMilli(),
	}); err != nil {
		t.Fatalf("create approval: %v", err)
	}

	for i, text := range []string{"/approve", "/deny", "/pending"} {
		if i == 0 {
			// Only the first text can settle the one pending question.
			if _, err := svc.SubmitChannelTask(ctx, domain.ChannelTaskInput{
				Scope: channelScope(), MessageID: "remote-ans-slash",
				QuestionID: question.ID, Parts: []string{text},
			}); err != nil {
				t.Fatalf("answer %q: %v", text, err)
			}
			continue
		}
		// Subsequent command-looking text targets a settled question: a
		// conflict is the only outcome — never a new effect.
		if _, err := svc.SubmitChannelTask(ctx, domain.ChannelTaskInput{
			Scope: channelScope(), MessageID: "remote-ans-slash-" + string(rune('0'+i)),
			QuestionID: question.ID, Parts: []string{text},
		}); err == nil {
			t.Fatalf("answer %q settled again", text)
		}
	}

	// The approval is untouched and still pending; the answer that looked
	// like a command never reached command parsing.
	approval, err := backend.GetApproval(ctx, "apr_guard")
	if err != nil || approval.Decision != domain.ApprovalPending {
		t.Fatalf("approval = %+v err=%v", approval, err)
	}
	if n := countEvents(t, backend, receipt.RunID, domain.EventToolApprovalDecided); n != 0 {
		t.Fatalf("approval decided events = %d", n)
	}
	// Using the approval id as a question selector discloses nothing.
	if _, err := svc.SubmitChannelTask(ctx, domain.ChannelTaskInput{
		Scope: channelScope(), MessageID: "remote-ans-apr",
		QuestionID: "apr_guard", Parts: []string{"yes"},
	}); !errors.Is(err, storage.ErrNotFound) {
		t.Fatalf("approval-id answer err = %v, want ErrNotFound", err)
	}
	if _, found, _ := backend.FindChannelTaskReceipt(ctx, channelScope(), "remote-ans-apr"); found {
		t.Fatal("approval-id answer left a receipt")
	}
}

// TestChannelTaskAnswerCrashRecovery: an answer committed durably before a
// process loss leaves the run suspended with no in-memory pending slot.
// Restart recovery must settle the run as one classified failure while
// retaining the answer evidence — never silently re-executing tools.
func TestChannelTaskAnswerCrashRecovery(t *testing.T) {
	svc, backend, _ := newChannelQuestionService(t)
	ctx := context.Background()
	t.Cleanup(func() { svc.CancelAll(); svc.WaitIdle(ctx) })

	receipt, question := admitQuestionRun(t, svc, backend, "remote-q4")

	// Commit the winning answer straight through the store (the durable
	// half of the transition) without the in-process resume handoff —
	// exactly the state a crash between commit and resume leaves.
	if _, err := backend.CommitChannelTaskAnswer(ctx, storage.ChannelTaskAnswerCommit{
		Scope: channelScope(), MessageID: "remote-ans-crash",
		InputHash: [32]byte{9},
		Transition: storage.QuestionTransitionCommit{
			RunID: question.RunID, QuestionID: question.ID,
			Outcome: domain.QuestionAnswered, Answer: "blue",
			Actor: "channel:a2a:p-1", At: time.Now().UnixMilli(),
			Event: domain.RunEvent{
				RunID: question.RunID, Type: domain.EventUserQuestionAnswered,
				CreatedAt: time.Now().UnixMilli(), PayloadVersion: 2,
				Payload: []byte(`{"question_id":"` + question.ID + `","answer":"blue","actor":"channel:a2a:p-1"}`),
			},
		},
	}); err != nil {
		t.Fatalf("commit answer: %v", err)
	}

	// Drop the in-memory pending slot to simulate the process losing it.
	svc.mu.Lock()
	delete(svc.pending, receipt.RunID)
	svc.mu.Unlock()

	if err := svc.Recover(ctx); err != nil {
		t.Fatalf("recover: %v", err)
	}
	waitForRunStatus(t, backend, receipt.RunID, domain.RunFailed)

	// Evidence retained: the question stays answered, the answered
	// envelope survives, and exactly one terminal classification was
	// appended — no silent tool re-execution, no second answered event.
	q, _ := backend.GetQuestion(ctx, question.ID)
	if q.Status != domain.QuestionAnswered || q.Answer != "blue" {
		t.Fatalf("question = %+v", q)
	}
	if n := countEvents(t, backend, receipt.RunID, domain.EventUserQuestionAnswered); n != 1 {
		t.Fatalf("answered events = %d", n)
	}
	if n := countEvents(t, backend, receipt.RunID, domain.EventRunFailed); n != 1 {
		t.Fatalf("terminal failures = %d, want exactly 1", n)
	}
	if n := countEvents(t, backend, receipt.RunID, domain.EventToolStarted); n != 0 {
		t.Fatalf("tool executions after crash = %d, want 0", n)
	}

	// A completed/failed/cancelled task is never reopened: answering the
	// settled question again is conflict, not resurrection.
	if _, err := svc.SubmitChannelTask(ctx, domain.ChannelTaskInput{
		Scope: channelScope(), MessageID: "remote-ans-reopen",
		QuestionID: question.ID, Parts: []string{"again"},
	}); err == nil {
		t.Fatal("settled run accepted a reopening answer")
	}
}

// TestChannelTaskAnswerConcurrentWithLocal races a remote answer against a
// local Review answer on the same pending question: exactly one settles.
func TestChannelTaskAnswerConcurrentWithLocal(t *testing.T) {
	svc, backend, _ := newChannelQuestionService(t)
	ctx := context.Background()
	t.Cleanup(func() { svc.CancelAll(); svc.WaitIdle(ctx) })

	receipt, question := admitQuestionRun(t, svc, backend, "remote-q5")

	var wg sync.WaitGroup
	start := make(chan struct{})
	errs := make([]error, 2)
	wg.Add(2)
	go func() {
		defer wg.Done()
		<-start
		errs[0] = svc.AnswerQuestion(ctx, question.ID, "local-blue")
	}()
	go func() {
		defer wg.Done()
		<-start
		_, errs[1] = svc.SubmitChannelTask(ctx, domain.ChannelTaskInput{
			Scope: channelScope(), MessageID: "remote-ans-race",
			SessionID: receipt.SessionID, RunID: receipt.RunID,
			QuestionID: question.ID, Parts: []string{"remote-blue"},
		})
	}()
	close(start)
	wg.Wait()

	wins := 0
	for _, err := range errs {
		if err == nil {
			wins++
		}
	}
	if wins != 1 {
		t.Fatalf("settlements = %d, errs=%v; want exactly one winner", wins, errs)
	}
	if n := countEvents(t, backend, receipt.RunID, domain.EventUserQuestionAnswered); n != 1 {
		t.Fatalf("answered events = %d", n)
	}
}
