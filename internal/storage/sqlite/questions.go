package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/storage"
)

// CreateQuestion inserts one pending user question.
func (b *Backend) CreateQuestion(ctx context.Context, q domain.Question) error {
	if _, err := b.db.ExecContext(ctx,
		`INSERT INTO questions (id, run_id, tool_call_id, prompt, answer, status, expires_at, resume_target)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		q.ID, q.RunID, q.ToolCallID, q.Prompt, q.Answer, q.Status, q.ExpiresAt, q.ResumeTarget); err != nil {
		return fmt.Errorf("storage: create question %s: %w", q.ID, err)
	}
	return nil
}

// GetQuestion loads one question; absent ids yield storage.ErrNotFound.
func (b *Backend) GetQuestion(ctx context.Context, id string) (domain.Question, error) {
	var q domain.Question
	var runID, status string
	err := b.db.QueryRowContext(ctx,
		`SELECT id, run_id, tool_call_id, prompt, answer, status, expires_at, resume_target
		 FROM questions WHERE id = ?`, id).
		Scan(&q.ID, &runID, &q.ToolCallID, &q.Prompt, &q.Answer, &status, &q.ExpiresAt, &q.ResumeTarget)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Question{}, storage.ErrNotFound
	}
	if err != nil {
		return domain.Question{}, fmt.Errorf("storage: get question %s: %w", id, err)
	}
	q.RunID = domain.RunID(runID)
	q.Status = domain.QuestionStatus(status)
	return q, nil
}

// ListPendingQuestions returns unresolved questions, newest expiry first.
func (b *Backend) ListPendingQuestions(ctx context.Context) ([]domain.Question, error) {
	rows, err := b.db.QueryContext(ctx,
		`SELECT id, run_id, tool_call_id, prompt, answer, status, expires_at, resume_target
		 FROM questions WHERE status = ? ORDER BY expires_at DESC, id`, domain.QuestionPending)
	if err != nil {
		return nil, fmt.Errorf("storage: list pending questions: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var out []domain.Question
	for rows.Next() {
		var q domain.Question
		var runID, status string
		if err := rows.Scan(&q.ID, &runID, &q.ToolCallID, &q.Prompt, &q.Answer, &status, &q.ExpiresAt, &q.ResumeTarget); err != nil {
			return nil, fmt.Errorf("storage: scan question: %w", err)
		}
		q.RunID = domain.RunID(runID)
		q.Status = domain.QuestionStatus(status)
		out = append(out, q)
	}
	return out, rows.Err()
}

// AnswerQuestion is first-writer-wins for a pending question.
func (b *Backend) AnswerQuestion(ctx context.Context, id, answer string) (bool, error) {
	res, err := b.db.ExecContext(ctx,
		`UPDATE questions SET answer = ?, status = ? WHERE id = ? AND status = ?`,
		answer, domain.QuestionAnswered, id, domain.QuestionPending)
	if err != nil {
		return false, fmt.Errorf("storage: answer question %s: %w", id, err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("storage: answer question %s: rows affected: %w", id, err)
	}
	return n > 0, nil
}

// CancelQuestion removes a question from the pending interaction surface
// when its run is explicitly cancelled.
func (b *Backend) CancelQuestion(ctx context.Context, id string) error {
	if _, err := b.db.ExecContext(ctx,
		`UPDATE questions SET status = ? WHERE id = ? AND status = ?`,
		domain.QuestionCancelled, id, domain.QuestionPending); err != nil {
		return fmt.Errorf("storage: cancel question %s: %w", id, err)
	}
	return nil
}
