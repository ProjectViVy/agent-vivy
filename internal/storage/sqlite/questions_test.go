package sqlite

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"agent-vivy/internal/domain"
)

func TestQuestionStoreIsDistinctAndFirstWriterWins(t *testing.T) {
	ctx := context.Background()
	b, err := Open(ctx, filepath.Join(t.TempDir(), "questions.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { _ = b.Close() })
	if err := b.CreateSession(ctx, domain.Session{ID: "sess-question", Title: "questions", CreatedAt: time.Now().UnixMilli()}); err != nil {
		t.Fatalf("create session: %v", err)
	}
	if err := b.CreateRun(ctx, domain.Run{ID: "run-question", SessionID: "sess-question", Status: domain.RunActive, CreatedAt: time.Now().UnixMilli()}); err != nil {
		t.Fatalf("create run: %v", err)
	}
	q := domain.Question{
		ID: "que_1", RunID: "run-question", ToolCallID: "call-1",
		Prompt: "pick a color", Status: domain.QuestionPending,
		ExpiresAt: time.Now().Add(time.Minute).UnixMilli(), ResumeTarget: "resume-1",
	}
	if err := b.CreateQuestion(ctx, q); err != nil {
		t.Fatalf("create question: %v", err)
	}
	if rows, err := b.ListPendingQuestions(ctx); err != nil || len(rows) != 1 || rows[0].ID != q.ID {
		t.Fatalf("pending questions = %+v, err=%v", rows, err)
	}
	if ok, err := b.AnswerQuestion(ctx, q.ID, "blue"); err != nil || !ok {
		t.Fatalf("answer question = %v, %v", ok, err)
	}
	if ok, err := b.AnswerQuestion(ctx, q.ID, "green"); err != nil || ok {
		t.Fatalf("second answer = %v, %v; want false", ok, err)
	}
	got, err := b.GetQuestion(ctx, q.ID)
	if err != nil {
		t.Fatalf("get question: %v", err)
	}
	if got.Status != domain.QuestionAnswered || got.Answer != "blue" {
		t.Fatalf("question = %+v", got)
	}
}
