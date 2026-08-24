package runtime

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/cloudwego/eino/schema"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/storage/sqlite"
	"agent-vivy/internal/tools"
)

func newQuestionService(t *testing.T) (*Service, *sqlite.Backend, *testSink) {
	t.Helper()
	ctx := context.Background()
	backend, err := sqlite.Open(ctx, filepath.Join(t.TempDir(), "questions.db"))
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
		ApprovalExpiration: 5 * time.Minute, Sink: sink,
	})
	return svc, backend, sink
}

func waitForPendingQuestion(t *testing.T, backend *sqlite.Backend, runID domain.RunID) domain.Question {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		questions, err := backend.ListPendingQuestions(context.Background())
		if err != nil {
			t.Fatalf("list pending questions: %v", err)
		}
		for _, q := range questions {
			if q.RunID == runID {
				return q
			}
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("run %s never produced a pending question", runID)
	return domain.Question{}
}

func TestServiceQuestionSuspendAnswerAndResume(t *testing.T) {
	svc, backend, _ := newQuestionService(t)
	ctx := context.Background()
	runID, err := svc.Run(ctx, "sess-question", "ask me for a color")
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	question := waitForPendingQuestion(t, backend, runID)
	if question.Prompt != "Which color should I use?" || question.ToolCallID != QuestionFlowCallID {
		t.Fatalf("question = %+v", question)
	}
	if approvals, err := backend.ListPendingApprovals(ctx); err != nil || len(approvals) != 0 {
		t.Fatalf("question must not create approvals: %v %+v", err, approvals)
	}

	if err := svc.AnswerQuestion(ctx, question.ID, "blue"); err != nil {
		t.Fatalf("answer: %v", err)
	}
	waitForRunStatus(t, backend, runID, domain.RunCompleted)
	answered, err := backend.GetQuestion(ctx, question.ID)
	if err != nil {
		t.Fatalf("get answered question: %v", err)
	}
	if answered.Status != domain.QuestionAnswered || answered.Answer != "blue" {
		t.Fatalf("answered question = %+v", answered)
	}
	if err := svc.AnswerQuestion(ctx, question.ID, "green"); err != ErrQuestionAlreadyAnswered {
		t.Fatalf("second answer error = %v, want ErrQuestionAlreadyAnswered", err)
	}

	events := replayAll(t, backend, runID)
	if indexOfType(events, domain.EventUserQuestionRequired) < 0 || indexOfType(events, domain.EventUserQuestionAnswered) < 0 {
		t.Fatalf("question lifecycle events missing: %v", events)
	}
	if n := countTerminal(events); n != 1 || events[len(events)-1].Type != domain.EventRunCompleted {
		t.Fatalf("terminal events = %d, last = %s", n, events[len(events)-1].Type)
	}
}

func restartQuestionService(t *testing.T, backend *sqlite.Backend) *Service {
	t.Helper()
	ctx := context.Background()
	ts, err := tools.Builtin(backend).Resolve([]string{tools.AskUserName})
	if err != nil {
		t.Fatalf("resolve ask_user: %v", err)
	}
	checkpoints, err := NewVersionedCheckpointStore(backend.Blobs(), "test-engine")
	if err != nil {
		t.Fatalf("checkpoint store: %v", err)
	}
	eng, err := NewEngine(ctx, NewScriptedModel(schema.AssistantMessage("Recovered answer.", nil)), ts, EngineConfig{
		StreamBuffer: 8, MaxEventPayloadBytes: 64 << 10, Checkpoints: checkpoints,
	})
	if err != nil {
		t.Fatalf("new engine: %v", err)
	}
	return NewService(eng, "scripted", "scripted-v0", ServiceDeps{
		Journal: backend, Runs: backend, Messages: backend, Questions: backend,
		ApprovalExpiration: 5 * time.Minute, Sink: newTestSink(),
	})
}

func TestServiceQuestionRecoveryKeepsQuestionDistinct(t *testing.T) {
	initial, backend, _ := newQuestionService(t)
	runID, err := initial.Run(context.Background(), "sess-recover-question", "ask me for a color")
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	question := waitForPendingQuestion(t, backend, runID)
	restarted := restartQuestionService(t, backend)
	if err := restarted.Recover(context.Background()); err != nil {
		t.Fatalf("recover: %v", err)
	}
	if err := restarted.AnswerQuestion(context.Background(), question.ID, "blue"); err != nil {
		t.Fatalf("answer after recovery: %v", err)
	}
	waitForRunStatus(t, backend, runID, domain.RunCompleted)
	if got, err := backend.GetQuestion(context.Background(), question.ID); err != nil || got.Status != domain.QuestionAnswered {
		t.Fatalf("recovered question = %+v, err=%v", got, err)
	}
}
