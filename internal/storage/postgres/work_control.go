package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/storage"
)

type workPayload struct {
	Mutation  domain.WorkMutation
	Admission domain.GoalRunAdmission
}

type workScanner interface {
	Scan(dest ...any) error
}

func (b *Backend) ReadWork(ctx context.Context, sessionID domain.SessionID) (domain.WorkState, error) {
	if _, err := b.GetSession(ctx, sessionID); err != nil {
		return domain.WorkState{}, err
	}
	events, err := b.readWorkEvents(ctx, sessionID, 0, 0)
	if err != nil {
		return domain.WorkState{}, err
	}
	if len(events) == 0 {
		return domain.WorkState{SessionID: sessionID}, nil
	}
	state, err := domain.FoldWork(events)
	if err != nil {
		return domain.WorkState{}, fmt.Errorf("storage: fold work %s: %w", sessionID, err)
	}
	return state, nil
}

func (b *Backend) ReplayWork(ctx context.Context, sessionID domain.SessionID, after domain.WorkVersion, limit int) ([]domain.WorkEvent, error) {
	if limit <= 0 {
		return []domain.WorkEvent{}, nil
	}
	events, err := b.readWorkEvents(ctx, sessionID, domain.WorkSeq(after), limit)
	if err != nil {
		return nil, err
	}
	want := domain.WorkSeq(after) + 1
	for _, event := range events {
		if event.Seq != want {
			return nil, fmt.Errorf("%w: got %d after %d", domain.ErrNonContiguousWorkSeq, event.Seq, want-1)
		}
		want++
	}
	return events, nil
}

func (b *Backend) CommitWork(ctx context.Context, mutation domain.WorkMutation) (storage.WorkCommitResult, error) {
	if err := storage.ValidateWorkMutation(mutation); err != nil {
		return storage.WorkCommitResult{}, err
	}
	tx, err := b.db.BeginTx(ctx, nil)
	if err != nil {
		return storage.WorkCommitResult{}, fmt.Errorf("storage: begin work commit: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	var lockedSession string
	if err := tx.QueryRowContext(ctx, "SELECT id FROM sessions WHERE id = ? FOR UPDATE",
		mutation.SessionID).Scan(&lockedSession); errors.Is(err, sql.ErrNoRows) {
		return storage.WorkCommitResult{}, storage.ErrNotFound
	} else if err != nil {
		return storage.WorkCommitResult{}, fmt.Errorf("storage: lock work session: %w", err)
	}

	rows, err := tx.SQL.QueryContext(ctx, rebind("SELECT session_id, work_seq, kind, payload_version, request_id, request_hash, created_at, payload FROM session_work_events WHERE session_id = ? ORDER BY work_seq"), mutation.SessionID)
	if err != nil {
		return storage.WorkCommitResult{}, fmt.Errorf("storage: read work events in transaction %s: %w", mutation.SessionID, err)
	}
	events, scanErr := scanWorkEvents(rows)
	_ = rows.Close()
	if scanErr != nil {
		return storage.WorkCommitResult{}, scanErr
	}
	for i, event := range events {
		if event.RequestID != mutation.RequestID {
			continue
		}
		if event.RequestHash != mutation.RequestHash {
			return storage.WorkCommitResult{}, storage.ErrWorkRequestConflict
		}
		state, err := domain.FoldWork(events[:i+1])
		if err != nil {
			return storage.WorkCommitResult{}, fmt.Errorf("storage: fold replayed work: %w", err)
		}
		return storage.WorkCommitResult{State: state, Event: event, Replayed: true}, nil
	}

	state := domain.WorkState{SessionID: mutation.SessionID}
	if len(events) > 0 {
		state, err = domain.FoldWork(events)
		if err != nil {
			return storage.WorkCommitResult{}, fmt.Errorf("storage: fold work: %w", err)
		}
	}
	if state.Version != mutation.ExpectedVersion {
		return storage.WorkCommitResult{}, storage.ErrWorkVersionConflict
	}

	event := domain.WorkEvent{
		SessionID:      mutation.SessionID,
		Seq:            domain.WorkSeq(state.Version + 1),
		Kind:           mutation.Kind,
		PayloadVersion: domain.WorkPayloadVersion,
		RequestID:      mutation.RequestID,
		RequestHash:    mutation.RequestHash,
		CreatedAt:      time.Now().UnixMilli(),
		Mutation:       mutation,
		Admission:      mutation.Admission,
	}
	candidate := append(append([]domain.WorkEvent(nil), events...), event)
	next, err := domain.FoldWork(candidate)
	if err != nil {
		return storage.WorkCommitResult{}, err
	}
	payload, err := json.Marshal(workPayload{Mutation: mutation, Admission: mutation.Admission})
	if err != nil {
		return storage.WorkCommitResult{}, fmt.Errorf("storage: encode work event: %w", err)
	}
	if _, err := tx.ExecContext(ctx,
		"INSERT INTO session_work_events (session_id, work_seq, kind, payload_version, request_id, request_hash, created_at, payload) VALUES (?, ?, ?, ?, ?, ?, ?, ?)",
		event.SessionID, int64(event.Seq), string(event.Kind), event.PayloadVersion,
		event.RequestID, event.RequestHash, event.CreatedAt, payload); err != nil {
		return storage.WorkCommitResult{}, fmt.Errorf("storage: insert work event: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return storage.WorkCommitResult{}, fmt.Errorf("storage: commit work event: %w", err)
	}
	return storage.WorkCommitResult{State: next, Event: event}, nil
}

func (b *Backend) readWorkEvents(ctx context.Context, sessionID domain.SessionID, after domain.WorkSeq, limit int) ([]domain.WorkEvent, error) {
	query := "SELECT session_id, work_seq, kind, payload_version, request_id, request_hash, created_at, payload FROM session_work_events WHERE session_id = ? AND work_seq > ? ORDER BY work_seq"
	args := []any{sessionID, int64(after)}
	if limit > 0 {
		query += " LIMIT ?"
		args = append(args, limit)
	}
	rows, err := b.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("storage: read work events %s: %w", sessionID, err)
	}
	defer func() { _ = rows.Close() }()
	return scanWorkEvents(rows)
}

func scanWorkEvents(scanner interface {
	Next() bool
	Scan(dest ...any) error
	Err() error
}) ([]domain.WorkEvent, error) {
	var events []domain.WorkEvent
	for scanner.Next() {
		event, err := scanWorkEvent(scanner)
		if err != nil {
			return nil, err
		}
		events = append(events, event)
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("storage: iterate work events: %w", err)
	}
	return events, nil
}

func scanWorkEvent(scanner workScanner) (domain.WorkEvent, error) {
	var sessionID, kind, requestID, requestHash string
	var seq, payloadVersion, createdAt int64
	var payload []byte
	if err := scanner.Scan(&sessionID, &seq, &kind, &payloadVersion, &requestID, &requestHash, &createdAt, &payload); err != nil {
		return domain.WorkEvent{}, fmt.Errorf("storage: scan work event: %w", err)
	}
	var decoded workPayload
	if err := json.Unmarshal(payload, &decoded); err != nil {
		return domain.WorkEvent{}, fmt.Errorf("%w: decode payload: %v", storage.ErrWorkEventCorrupt, err)
	}
	if sessionID == "" || requestID == "" || requestHash == "" || !domain.WorkEventKind(kind).Valid() {
		return domain.WorkEvent{}, storage.ErrWorkEventCorrupt
	}
	return domain.WorkEvent{
		SessionID:      domain.SessionID(sessionID),
		Seq:            domain.WorkSeq(seq),
		Kind:           domain.WorkEventKind(kind),
		PayloadVersion: int(payloadVersion),
		RequestID:      requestID,
		RequestHash:    requestHash,
		CreatedAt:      createdAt,
		Mutation:       decoded.Mutation,
		Admission:      decoded.Admission,
	}, nil
}
