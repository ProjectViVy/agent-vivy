package postgres

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"fmt"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/storage"
)

// postgresCommitQuestionTransition mirrors sqliteCommitQuestionTransition:
// the caller already holds the transaction and has taken the run-level
// advisory lock, so this performs scope→question→terminal→CAS→event in
// that order.
func postgresCommitQuestionTransition(ctx context.Context, tx *Tx, c storage.QuestionTransitionCommit) (domain.SessionID, domain.EventSeq, error) {
	var (
		status    string
		runID     string
		expiresAt int64
		sessionID string
	)
	err := tx.QueryRowContext(ctx,
		`SELECT q.status, q.run_id, q.expires_at, r.session_id
		 FROM questions q JOIN runs r ON r.id = q.run_id
		 WHERE q.id = ? FOR UPDATE`, c.QuestionID).
		Scan(&status, &runID, &expiresAt, &sessionID)
	if errors.Is(err, sql.ErrNoRows) {
		return "", 0, storage.ErrNotFound
	}
	if err != nil {
		return "", 0, fmt.Errorf("storage: read question %s: %w", c.QuestionID, err)
	}
	if domain.RunID(runID) != c.RunID {
		return "", 0, storage.ErrNotFound
	}
	if domain.QuestionStatus(status) != domain.QuestionPending {
		return "", 0, storage.ErrConflict
	}
	if expiresAt > 0 && c.At >= expiresAt {
		return "", 0, storage.ErrConflict
	}

	var closed int
	if err := tx.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM run_events WHERE run_id = ? AND type IN `+terminalTypes,
		c.RunID).Scan(&closed); err != nil {
		return "", 0, fmt.Errorf("storage: check terminal: %w", err)
	}
	if closed > 0 {
		return "", 0, storage.ErrRunClosed
	}

	res, err := tx.ExecContext(ctx,
		`UPDATE questions SET answer = ?, status = ?, answered_at = ?, actor = ?, decision_reason = ?
		 WHERE id = ? AND status = ?`,
		c.Answer, string(c.Outcome), c.At, c.Actor, c.Reason, c.QuestionID, string(domain.QuestionPending))
	if err != nil {
		return "", 0, fmt.Errorf("storage: settle question %s: %w", c.QuestionID, err)
	}
	if n, err := res.RowsAffected(); err != nil || n == 0 {
		return "", 0, storage.ErrConflict
	}

	var maxSeq sql.NullInt64
	if err := tx.QueryRowContext(ctx,
		`SELECT MAX(seq) FROM run_events WHERE run_id = ?`, c.RunID).Scan(&maxSeq); err != nil {
		return "", 0, fmt.Errorf("storage: read max seq: %w", err)
	}
	event := c.Event
	event.Seq = domain.EventSeq(maxSeq.Int64 + 1)
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO run_events (run_id, seq, type, created_at, payload_version, payload) VALUES (?, ?, ?, ?, ?, ?)`,
		event.RunID, event.Seq, string(event.Type), event.CreatedAt, event.PayloadVersion, event.Payload); err != nil {
		return "", 0, fmt.Errorf("storage: journal question transition: %w", err)
	}
	return domain.SessionID(sessionID), event.Seq, nil
}

// CommitQuestionTransition implements storage.QuestionTransitionStore for
// the local native caller (answer/cancel/expiry). The run-level advisory
// lock serializes this transaction with every Journal.Append on the run.
func (b *Backend) CommitQuestionTransition(ctx context.Context, c storage.QuestionTransitionCommit) (storage.QuestionTransitionResult, error) {
	if c.QuestionID == "" || c.RunID == "" || c.Event.RunID != c.RunID || c.At <= 0 {
		return storage.QuestionTransitionResult{}, storage.ErrWorkInvalidMutation
	}
	tx, err := b.db.BeginTx(ctx, nil)
	if err != nil {
		return storage.QuestionTransitionResult{}, fmt.Errorf("storage: begin question transition: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(hashtextextended(?, 0))`, string(c.RunID)); err != nil {
		return storage.QuestionTransitionResult{}, fmt.Errorf("storage: lock run %s: %w", c.RunID, err)
	}
	_, seq, err := postgresCommitQuestionTransition(ctx, tx, c)
	if err != nil {
		return storage.QuestionTransitionResult{}, err
	}
	if err := tx.Commit(); err != nil {
		return storage.QuestionTransitionResult{}, fmt.Errorf("storage: commit question transition: %w", err)
	}
	event := c.Event
	event.Seq = seq
	return storage.QuestionTransitionResult{Changed: true, Events: []domain.RunEvent{event}}, nil
}

// CommitChannelTaskAnswer implements the answer half of
// storage.ChannelTaskStore: run advisory lock -> scope lock -> receipt
// pre-check -> question CAS -> answered event -> receipt.
func (b *Backend) CommitChannelTaskAnswer(ctx context.Context, c storage.ChannelTaskAnswerCommit) (storage.ChannelTaskCommitResult, error) {
	scope := c.Scope
	if scope.InstanceKey == "" || scope.PrincipalID == "" || c.MessageID == "" ||
		c.Transition.QuestionID == "" || c.Transition.RunID == "" || c.Transition.At <= 0 ||
		c.Transition.Outcome != domain.QuestionAnswered {
		return storage.ChannelTaskCommitResult{}, storage.ErrWorkInvalidMutation
	}
	tx, err := b.db.BeginTx(ctx, nil)
	if err != nil {
		return storage.ChannelTaskCommitResult{}, fmt.Errorf("storage: begin channel task answer: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	// Serialize with Journal.Append on this run first, then the scope lock.
	if _, err := tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(hashtextextended(?, 0))`, string(c.Transition.RunID)); err != nil {
		return storage.ChannelTaskCommitResult{}, fmt.Errorf("storage: lock run %s: %w", c.Transition.RunID, err)
	}
	var scopeInstance string
	if err := tx.QueryRowContext(ctx,
		`SELECT instance_key FROM channel_task_scopes WHERE instance_key = ? AND principal_id = ? FOR UPDATE`,
		scope.InstanceKey, scope.PrincipalID).Scan(&scopeInstance); errors.Is(err, sql.ErrNoRows) {
		return storage.ChannelTaskCommitResult{}, storage.ErrNotFound
	} else if err != nil {
		return storage.ChannelTaskCommitResult{}, fmt.Errorf("storage: lock channel task scope: %w", err)
	}

	var storedHash []byte
	var receipt domain.ChannelTaskReceipt
	var questionID sql.NullString
	var deletedAt sql.NullInt64
	err = tx.QueryRowContext(ctx,
		`SELECT input_hash, operation, session_id, run_id, question_id, accepted_seq, created_at, deleted_at
		 FROM channel_task_receipts
		 WHERE instance_key = ? AND principal_id = ? AND message_id = ?`,
		scope.InstanceKey, scope.PrincipalID, c.MessageID).
		Scan(&storedHash, &receipt.Operation, &receipt.SessionID, &receipt.RunID, &questionID,
			&receipt.AcceptedSeq, &receipt.CreatedAt, &deletedAt)
	reviveReceipt := false
	switch {
	case err == nil:
		if !deletedAt.Valid {
			if !bytes.Equal(storedHash, c.InputHash[:]) {
				return storage.ChannelTaskCommitResult{}, storage.ErrConflict
			}
			if err := tx.Commit(); err != nil {
				return storage.ChannelTaskCommitResult{}, fmt.Errorf("storage: replay channel task answer: %w", err)
			}
			receipt.Scope = scope
			receipt.MessageID = c.MessageID
			receipt.InputHash = c.InputHash
			receipt.QuestionID = questionID.String
			return storage.ChannelTaskCommitResult{Receipt: receipt, NewlyCommitted: false}, nil
		}
		reviveReceipt = true
	case !errors.Is(err, sql.ErrNoRows):
		return storage.ChannelTaskCommitResult{}, fmt.Errorf("storage: read channel task receipt: %w", err)
	}

	sessionID, seq, err := postgresCommitQuestionTransition(ctx, tx, c.Transition)
	if err != nil {
		return storage.ChannelTaskCommitResult{}, err
	}
	createdAt := c.Transition.At

	if reviveReceipt {
		if _, err := tx.ExecContext(ctx,
			`UPDATE channel_task_receipts SET input_hash = ?, operation = 'answer', session_id = ?, run_id = ?, question_id = ?, accepted_seq = ?, created_at = ?, deleted_at = NULL
			 WHERE instance_key = ? AND principal_id = ? AND message_id = ?`,
			c.InputHash[:], sessionID, c.Transition.RunID, c.Transition.QuestionID, seq, createdAt,
			scope.InstanceKey, scope.PrincipalID, c.MessageID); err != nil {
			return storage.ChannelTaskCommitResult{}, fmt.Errorf("storage: revive channel task answer receipt: %w", err)
		}
	} else if _, err := tx.ExecContext(ctx,
		`INSERT INTO channel_task_receipts (instance_key, principal_id, message_id, input_hash, operation, session_id, run_id, question_id, accepted_seq, created_at)
		 VALUES (?, ?, ?, ?, 'answer', ?, ?, ?, ?, ?)`,
		scope.InstanceKey, scope.PrincipalID, c.MessageID, c.InputHash[:],
		sessionID, c.Transition.RunID, c.Transition.QuestionID, seq, createdAt); err != nil {
		return storage.ChannelTaskCommitResult{}, fmt.Errorf("storage: write channel task answer receipt: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return storage.ChannelTaskCommitResult{}, fmt.Errorf("storage: commit channel task answer: %w", err)
	}
	event := c.Transition.Event
	event.Seq = seq
	receipt = domain.ChannelTaskReceipt{
		Scope: scope, MessageID: c.MessageID, Operation: "answer",
		InputHash: c.InputHash, SessionID: sessionID,
		RunID: c.Transition.RunID, QuestionID: c.Transition.QuestionID,
		AcceptedSeq: seq, CreatedAt: createdAt,
	}
	return storage.ChannelTaskCommitResult{Receipt: receipt, Events: []domain.RunEvent{event}, NewlyCommitted: true}, nil
}
