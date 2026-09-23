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

// CommitGoalRun persists the user message, active run, run.started event and
// goal.round_admitted event in one transaction. Publication and engine drive
// happen only after this method returns successfully.
func (b *Backend) CommitGoalRun(ctx context.Context, admission storage.GoalRunCommit) (storage.GoalRunCommitResult, error) {
	if err := storage.ValidateGoalRunCommit(admission); err != nil {
		return storage.GoalRunCommitResult{}, err
	}
	tx, err := b.db.BeginTx(ctx, nil)
	if err != nil {
		return storage.GoalRunCommitResult{}, fmt.Errorf("storage: begin goal run admission: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	var sessionID string
	if err := tx.QueryRowContext(ctx, "SELECT id FROM sessions WHERE id = ? FOR UPDATE", admission.Mutation.SessionID).Scan(&sessionID); errors.Is(err, sql.ErrNoRows) {
		return storage.GoalRunCommitResult{}, storage.ErrNotFound
	} else if err != nil {
		return storage.GoalRunCommitResult{}, fmt.Errorf("storage: lock goal session: %w", err)
	}

	rows, err := tx.SQL.QueryContext(ctx, rebind("SELECT session_id, work_seq, kind, payload_version, request_id, request_hash, created_at, payload FROM session_work_events WHERE session_id = ? ORDER BY work_seq"), admission.Mutation.SessionID)
	if err != nil {
		return storage.GoalRunCommitResult{}, fmt.Errorf("storage: read goal work in transaction: %w", err)
	}
	events, scanErr := scanWorkEvents(rows)
	_ = rows.Close()
	if scanErr != nil {
		return storage.GoalRunCommitResult{}, scanErr
	}
	for i, event := range events {
		if event.RequestID != admission.Mutation.RequestID {
			continue
		}
		if event.RequestHash != admission.Mutation.RequestHash {
			return storage.GoalRunCommitResult{}, storage.ErrWorkRequestConflict
		}
		state, err := domain.FoldWork(events[:i+1])
		if err != nil {
			return storage.GoalRunCommitResult{}, fmt.Errorf("storage: fold replayed goal admission: %w", err)
		}
		run, err := readGoalRunTx(ctx, tx, event.Admission.RunID)
		if err != nil {
			return storage.GoalRunCommitResult{}, err
		}
		started, err := readGoalStartedTx(ctx, tx, event.Admission.RunID)
		if err != nil {
			return storage.GoalRunCommitResult{}, err
		}
		return storage.GoalRunCommitResult{
			Work:    storage.WorkCommitResult{State: state, Event: event, Replayed: true},
			Run:     run,
			Started: started,
		}, nil
	}

	state := domain.WorkState{SessionID: admission.Mutation.SessionID}
	if len(events) > 0 {
		state, err = domain.FoldWork(events)
		if err != nil {
			return storage.GoalRunCommitResult{}, fmt.Errorf("storage: fold goal work: %w", err)
		}
	}
	if state.Version != admission.Mutation.ExpectedVersion {
		return storage.GoalRunCommitResult{}, storage.ErrWorkVersionConflict
	}
	candidate := domain.WorkEvent{
		SessionID:      admission.Mutation.SessionID,
		Seq:            domain.WorkSeq(state.Version + 1),
		Kind:           domain.WorkEventGoalRoundAdmitted,
		PayloadVersion: domain.WorkPayloadVersion,
		RequestID:      admission.Mutation.RequestID,
		RequestHash:    admission.Mutation.RequestHash,
		CreatedAt:      admission.Started.CreatedAt,
		Mutation:       admission.Mutation,
		Admission:      admission.Mutation.Admission,
	}
	if candidate.CreatedAt <= 0 {
		candidate.CreatedAt = time.Now().UnixMilli()
	}
	next, err := domain.FoldWork(append(append([]domain.WorkEvent(nil), events...), candidate))
	if err != nil {
		return storage.GoalRunCommitResult{}, err
	}

	var active int
	if err := tx.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM runs WHERE session_id = ? AND kind = ? AND status IN "+activeStatuses,
		admission.Mutation.SessionID, string(domain.RunKindPrimary)).Scan(&active); err != nil {
		return storage.GoalRunCommitResult{}, fmt.Errorf("storage: inspect active goal run: %w", err)
	}
	if active != 0 {
		return storage.GoalRunCommitResult{}, storage.ErrWorkRunConflict
	}

	message := admission.Message
	message.WorkSeq = domain.WorkSeq(state.Version)
	if _, err := tx.ExecContext(ctx,
		"INSERT INTO messages (id, session_id, run_id, role, created_at, work_seq, content, tool_call_id, tool_name, tool_args, source, channel, chat_id, channel_message_id) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)",
		message.ID, message.SessionID, message.RunID, string(message.Role), message.CreatedAt, int64(message.WorkSeq), message.Content,
		message.ToolCallID, message.ToolName, toolArgsBlob(message.ToolArgs),
		message.Source, message.Channel, message.ChatID, message.ChannelMessageID); err != nil {
		return storage.GoalRunCommitResult{}, fmt.Errorf("storage: append goal message: %w", err)
	}
	for position, attachment := range message.Attachments {
		if _, err := tx.ExecContext(ctx,
			"INSERT INTO message_attachments (message_id, position, name, mime_type, data) VALUES (?, ?, ?, ?, ?)",
			message.ID, position, attachment.Name, attachment.MimeType, attachment.Data); err != nil {
			return storage.GoalRunCommitResult{}, fmt.Errorf("storage: append goal attachment: %w", err)
		}
	}
	for position, file := range message.FileContexts {
		if _, err := tx.ExecContext(ctx,
			"INSERT INTO message_file_contexts (message_id, position, path, name, size, content) VALUES (?, ?, ?, ?, ?, ?)",
			message.ID, position, file.Path, file.Name, file.Size, file.Content); err != nil {
			return storage.GoalRunCommitResult{}, fmt.Errorf("storage: append goal file context: %w", err)
		}
	}
	at := messageActivityAt(message.CreatedAt)
	if _, err := tx.ExecContext(ctx,
		"UPDATE sessions SET updated_at = CASE WHEN updated_at < ? THEN ? ELSE updated_at END WHERE id = ?",
		at, at, message.SessionID); err != nil {
		return storage.GoalRunCommitResult{}, fmt.Errorf("storage: touch goal session: %w", err)
	}

	run := admission.Run
	kind := run.Kind
	if !kind.Valid() {
		kind = domain.RunKindPrimary
	}
	rootID := run.RootID
	if rootID == "" {
		rootID = run.ID
	}
	if _, err := tx.ExecContext(ctx,
		"INSERT INTO runs (id, session_id, status, created_at, kind, parent_run_id, root_run_id, depth) VALUES (?, ?, ?, ?, ?, ?, ?, ?)",
		run.ID, run.SessionID, string(run.Status), run.CreatedAt, string(kind), run.ParentID, rootID, run.Depth); err != nil {
		return storage.GoalRunCommitResult{}, fmt.Errorf("storage: create goal run: %w", err)
	}

	started := admission.Started
	started.Seq = 1
	if _, err := tx.ExecContext(ctx,
		"INSERT INTO run_events (run_id, seq, type, created_at, payload_version, payload) VALUES (?, ?, ?, ?, ?, ?)",
		started.RunID, int64(started.Seq), string(started.Type), started.CreatedAt, started.PayloadVersion, started.Payload); err != nil {
		return storage.GoalRunCommitResult{}, fmt.Errorf("storage: append goal run.started: %w", err)
	}

	payload, err := json.Marshal(workPayload{Mutation: candidate.Mutation, Admission: candidate.Admission})
	if err != nil {
		return storage.GoalRunCommitResult{}, fmt.Errorf("storage: encode goal admission: %w", err)
	}
	if _, err := tx.ExecContext(ctx,
		"INSERT INTO session_work_events (session_id, work_seq, kind, payload_version, request_id, request_hash, created_at, payload) VALUES (?, ?, ?, ?, ?, ?, ?, ?)",
		candidate.SessionID, int64(candidate.Seq), string(candidate.Kind), candidate.PayloadVersion,
		candidate.RequestID, candidate.RequestHash, candidate.CreatedAt, payload); err != nil {
		return storage.GoalRunCommitResult{}, fmt.Errorf("storage: insert goal admission: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return storage.GoalRunCommitResult{}, fmt.Errorf("storage: commit goal run admission: %w", err)
	}
	run.Kind = kind
	run.RootID = rootID
	return storage.GoalRunCommitResult{
		Work:    storage.WorkCommitResult{State: next, Event: candidate},
		Run:     run,
		Started: started,
	}, nil
}

func readGoalRunTx(ctx context.Context, tx *Tx, id domain.RunID) (domain.Run, error) {
	var run domain.Run
	var runID, sessionID, status, kind, parentID, rootID string
	err := tx.QueryRowContext(ctx,
		"SELECT id, session_id, status, created_at, kind, parent_run_id, root_run_id, depth FROM runs WHERE id = ?", id).
		Scan(&runID, &sessionID, &status, &run.CreatedAt, &kind, &parentID, &rootID, &run.Depth)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Run{}, storage.ErrNotFound
	}
	if err != nil {
		return domain.Run{}, fmt.Errorf("storage: read admitted run: %w", err)
	}
	run.ID, run.SessionID, run.Status = domain.RunID(runID), domain.SessionID(sessionID), domain.RunStatus(status)
	run.Kind, run.ParentID, run.RootID = domain.RunKind(kind), domain.RunID(parentID), domain.RunID(rootID)
	return run, nil
}

func readGoalStartedTx(ctx context.Context, tx *Tx, id domain.RunID) (domain.RunEvent, error) {
	rows, err := tx.SQL.QueryContext(ctx, rebind("SELECT run_id, seq, type, created_at, payload_version, payload FROM run_events WHERE run_id = ? ORDER BY seq LIMIT 1"), id)
	if err != nil {
		return domain.RunEvent{}, fmt.Errorf("storage: read admitted run.started: %w", err)
	}
	defer func() { _ = rows.Close() }()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return domain.RunEvent{}, err
		}
		return domain.RunEvent{}, storage.ErrWorkEventCorrupt
	}
	var event domain.RunEvent
	var runID, typ string
	var seq int64
	if err := rows.Scan(&runID, &seq, &typ, &event.CreatedAt, &event.PayloadVersion, &event.Payload); err != nil {
		return domain.RunEvent{}, fmt.Errorf("storage: scan admitted run.started: %w", err)
	}
	event.RunID, event.Seq, event.Type = domain.RunID(runID), domain.EventSeq(seq), domain.EventType(typ)
	return event, nil
}
