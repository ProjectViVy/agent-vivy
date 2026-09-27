package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sort"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/storage"
)

var _ storage.ContinuityStore = (*Backend)(nil)

// postgresContinuitySessions returns the sorted, deduplicated set of
// sessions the admission transaction must lock: destination plus every
// referenced source. Locking always walks this sorted order so concurrent
// admission, source deletion and workspace checks cannot interlock.
func postgresContinuitySessions(a storage.ContinuityAdmission) []domain.SessionID {
	seen := map[domain.SessionID]struct{}{a.SessionID: {}}
	for _, id := range a.Scope.SourceSessionIDs {
		seen[id] = struct{}{}
	}
	for _, e := range a.Expectations {
		seen[e.SourceSessionID] = struct{}{}
	}
	ids := make([]domain.SessionID, 0, len(seen))
	for id := range seen {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	return ids
}

func postgresLockContinuitySessions(ctx context.Context, tx *sql.Tx, a storage.ContinuityAdmission) error {
	sources := make(map[domain.SessionID]struct{}, len(a.Scope.SourceSessionIDs)+len(a.Expectations))
	for _, id := range a.Scope.SourceSessionIDs {
		sources[id] = struct{}{}
	}
	for _, e := range a.Expectations {
		sources[e.SourceSessionID] = struct{}{}
	}
	for _, id := range postgresContinuitySessions(a) {
		var found string
		err := tx.QueryRowContext(ctx, `SELECT id FROM sessions WHERE id = $1 FOR UPDATE`, id).Scan(&found)
		switch {
		case err == nil:
		case errors.Is(err, sql.ErrNoRows) && id == a.SessionID:
			return storage.ErrNotFound
		case errors.Is(err, sql.ErrNoRows):
			if _, isSource := sources[id]; isSource {
				return storage.ErrSourceChanged
			}
			return storage.ErrNotFound
		default:
			return fmt.Errorf("storage: lock continuity session %s: %w", id, err)
		}
	}
	return nil
}

func postgresFindReceiptTx(ctx context.Context, tx *sql.Tx, sessionID domain.SessionID, operation, requestID string) (domain.ContinuityReceipt, bool, error) {
	var r domain.ContinuityReceipt
	var seq int64
	err := tx.QueryRowContext(ctx,
		`SELECT session_id, run_id, operation, request_id, input_hash, event_seq FROM continuity_receipts
		 WHERE session_id = $1 AND operation = $2 AND request_id = $3`,
		sessionID, operation, requestID).
		Scan((*string)(&r.SessionID), (*string)(&r.RunID), &r.Operation, &r.RequestID, &r.InputHash, &seq)
	switch {
	case err == nil:
		r.EventSeq = domain.EventSeq(seq)
		return r, true, nil
	case errors.Is(err, sql.ErrNoRows):
		return domain.ContinuityReceipt{}, false, nil
	default:
		return domain.ContinuityReceipt{}, false, fmt.Errorf("storage: find continuity receipt: %w", err)
	}
}

func (b *Backend) FindContinuityReceipt(ctx context.Context, sessionID domain.SessionID, operation, requestID string) (domain.ContinuityReceipt, bool, error) {
	var r domain.ContinuityReceipt
	var seq int64
	err := b.db.QueryRowContext(ctx,
		`SELECT session_id, run_id, operation, request_id, input_hash, event_seq FROM continuity_receipts
		 WHERE session_id = ? AND operation = ? AND request_id = ?`,
		sessionID, operation, requestID).
		Scan((*string)(&r.SessionID), (*string)(&r.RunID), &r.Operation, &r.RequestID, &r.InputHash, &seq)
	switch {
	case err == nil:
		r.EventSeq = domain.EventSeq(seq)
		return r, true, nil
	case errors.Is(err, sql.ErrNoRows):
		return domain.ContinuityReceipt{}, false, nil
	default:
		return domain.ContinuityReceipt{}, false, fmt.Errorf("storage: find continuity receipt: %w", err)
	}
}

// postgresReplayAdmission loads the originally committed startup events —
// the contiguous prefix ending at the receipt's event_seq — so a lost
// response returns the same committed startup view.
func postgresReplayAdmission(ctx context.Context, tx *sql.Tx, receipt domain.ContinuityReceipt) (storage.ContinuityResult, error) {
	rows, err := tx.QueryContext(ctx,
		`SELECT run_id, seq, type, created_at, payload_version, payload FROM run_events
		 WHERE run_id = $1 AND seq <= $2 ORDER BY seq`, receipt.RunID, int64(receipt.EventSeq))
	if err != nil {
		return storage.ContinuityResult{}, fmt.Errorf("storage: replay continuity admission: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var events []domain.RunEvent
	for rows.Next() {
		var runID, typ string
		var e domain.RunEvent
		if err := rows.Scan(&runID, (*int64)(&e.Seq), &typ, &e.CreatedAt, &e.PayloadVersion, &e.Payload); err != nil {
			return storage.ContinuityResult{}, fmt.Errorf("storage: scan continuity admission event: %w", err)
		}
		e.RunID = domain.RunID(runID)
		e.Type = domain.EventType(typ)
		events = append(events, e)
	}
	if err := rows.Err(); err != nil {
		return storage.ContinuityResult{}, fmt.Errorf("storage: replay continuity admission: %w", err)
	}
	return storage.ContinuityResult{RunID: receipt.RunID, Events: events, Receipt: receipt, NewlyCommitted: false}, nil
}

func postgresCheckExpectations(ctx context.Context, tx *sql.Tx, expectations []storage.SourceExpectation) error {
	for _, e := range expectations {
		if e.TruncationMarkers < 0 {
			continue
		}
		var count int64
		if err := tx.QueryRowContext(ctx,
			`SELECT COUNT(*) FROM session_truncations WHERE session_id = $1 AND reason IN ('rewind','edit')`,
			e.SourceSessionID).Scan(&count); err != nil {
			return fmt.Errorf("storage: check source truncation revision %s: %w", e.SourceSessionID, err)
		}
		if count != e.TruncationMarkers {
			return storage.ErrSourceChanged
		}
	}
	return nil
}

func postgresInsertContinuityEvent(ctx context.Context, tx *sql.Tx, e *domain.RunEvent, seq int64) error {
	e.Seq = domain.EventSeq(seq)
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO run_events (run_id, seq, type, created_at, payload_version, payload) VALUES ($1,$2,$3,$4,$5,$6)`,
		e.RunID, seq, e.Type, e.CreatedAt, e.PayloadVersion, e.Payload); err != nil {
		return fmt.Errorf("storage: insert continuity event %s seq %d: %w", e.Type, seq, err)
	}
	return nil
}

func postgresInsertMessageTx(ctx context.Context, tx *sql.Tx, m domain.Message) error {
	position, err := postgresNextMessagePosition(ctx, tx, m.SessionID)
	if err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO messages (id, session_id, run_id, role, created_at, content, tool_call_id, tool_name, tool_args, source, channel, chat_id, channel_message_id, position)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14)`,
		m.ID, m.SessionID, m.RunID, string(m.Role), m.CreatedAt, m.Content,
		m.ToolCallID, m.ToolName, toolArgsBlob(m.ToolArgs),
		m.Source, m.Channel, m.ChatID, m.ChannelMessageID, position); err != nil {
		return fmt.Errorf("storage: insert continuity message %s: %w", m.ID, err)
	}
	for i, attachment := range m.Attachments {
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO message_attachments (message_id, position, name, mime_type, data) VALUES ($1,$2,$3,$4,$5)`,
			m.ID, i, attachment.Name, attachment.MimeType, attachment.Data); err != nil {
			return fmt.Errorf("storage: insert continuity message attachment %d: %w", i, err)
		}
	}
	for i, file := range m.FileContexts {
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO message_file_contexts (message_id, position, path, name, size, content) VALUES ($1,$2,$3,$4,$5,$6)`,
			m.ID, i, file.Path, file.Name, file.Size, file.Content); err != nil {
			return fmt.Errorf("storage: insert continuity message file context %d: %w", i, err)
		}
	}
	return nil
}

func postgresFault(hook func(string) error, step string) error {
	if hook == nil {
		return nil
	}
	return hook(step)
}

// CommitContinuityRun commits the complete task admission atomically under
// sorted session row locks.
func (b *Backend) CommitContinuityRun(ctx context.Context, a storage.ContinuityAdmission) (storage.ContinuityResult, error) {
	if err := a.Validate(); err != nil {
		return storage.ContinuityResult{}, err
	}
	tx, err := b.db.SQL.BeginTx(ctx, nil)
	if err != nil {
		return storage.ContinuityResult{}, fmt.Errorf("storage: begin continuity admission: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	if err := postgresLockContinuitySessions(ctx, tx, a); err != nil {
		return storage.ContinuityResult{}, err
	}

	// Receipt lookup precedes every write so an identical replay returns the
	// original run without a second admission.
	existing, found, err := postgresFindReceiptTx(ctx, tx, a.SessionID, a.Receipt.Operation, a.Receipt.RequestID)
	if err != nil {
		return storage.ContinuityResult{}, err
	}
	if found {
		if existing.InputHash != a.Receipt.InputHash {
			return storage.ContinuityResult{}, storage.ErrConflict
		}
		return postgresReplayAdmission(ctx, tx, existing)
	}

	if err := postgresCheckExpectations(ctx, tx, a.Expectations); err != nil {
		return storage.ContinuityResult{}, err
	}
	if err := postgresInsertMessageTx(ctx, tx, a.Message); err != nil {
		return storage.ContinuityResult{}, err
	}
	if err := postgresFault(a.TestFaultHook, "message"); err != nil {
		return storage.ContinuityResult{}, err
	}
	kind := a.Run.Kind
	if kind == "" {
		kind = domain.RunKindPrimary
	}
	rootID := a.Run.RootID
	if rootID == "" {
		rootID = a.Run.ID
	}
	status := a.Run.Status
	if status == "" {
		status = domain.RunActive
	}
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO runs (id, session_id, status, created_at, kind, parent_run_id, root_run_id, depth) VALUES ($1,$2,$3,$4,$5,$6,$7,$8)`,
		a.Run.ID, a.Run.SessionID, string(status), a.Run.CreatedAt, string(kind), a.Run.ParentID, rootID, a.Run.Depth); err != nil {
		return storage.ContinuityResult{}, fmt.Errorf("storage: insert continuity run %s: %w", a.Run.ID, err)
	}
	if err := postgresFault(a.TestFaultHook, "run"); err != nil {
		return storage.ContinuityResult{}, err
	}
	events := append([]domain.RunEvent(nil), a.Events...)
	for i := range events {
		if err := postgresInsertContinuityEvent(ctx, tx, &events[i], int64(i+1)); err != nil {
			return storage.ContinuityResult{}, err
		}
	}
	if err := postgresFault(a.TestFaultHook, "events"); err != nil {
		return storage.ContinuityResult{}, err
	}
	receipt := a.Receipt
	receipt.RunID = a.Run.ID
	receipt.EventSeq = domain.EventSeq(len(events))
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO continuity_receipts (session_id, operation, request_id, input_hash, run_id, event_seq) VALUES ($1,$2,$3,$4,$5,$6)`,
		receipt.SessionID, receipt.Operation, receipt.RequestID, receipt.InputHash, receipt.RunID, int64(receipt.EventSeq)); err != nil {
		return storage.ContinuityResult{}, fmt.Errorf("storage: insert continuity receipt: %w", err)
	}
	if err := postgresFault(a.TestFaultHook, "receipt"); err != nil {
		return storage.ContinuityResult{}, err
	}
	at := messageActivityAt(a.Message.CreatedAt)
	if _, err := tx.ExecContext(ctx,
		`UPDATE sessions SET updated_at = CASE WHEN updated_at < $1 THEN $2 ELSE updated_at END WHERE id = $3`,
		at, at, a.SessionID); err != nil {
		return storage.ContinuityResult{}, fmt.Errorf("storage: touch admitted session: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return storage.ContinuityResult{}, fmt.Errorf("storage: commit continuity admission: %w", storage.ErrCommitUncertain)
	}
	return storage.ContinuityResult{RunID: a.Run.ID, Events: events, Receipt: receipt, NewlyCommitted: true}, nil
}

// CommitContinuityOperation appends one guarded event plus its receipt. The
// receipt lookup precedes the terminal check so an identical replay still
// returns the original result after the owning run finished.
func (b *Backend) CommitContinuityOperation(ctx context.Context, m storage.ContinuityMutation) (storage.ContinuityResult, error) {
	if err := m.Validate(); err != nil {
		return storage.ContinuityResult{}, err
	}
	tx, err := b.db.SQL.BeginTx(ctx, nil)
	if err != nil {
		return storage.ContinuityResult{}, fmt.Errorf("storage: begin continuity operation: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	var owner string
	err = tx.QueryRowContext(ctx, `SELECT session_id FROM runs WHERE id = $1 FOR UPDATE`, m.RunID).Scan(&owner)
	switch {
	case err == nil && owner != string(m.SessionID):
		return storage.ContinuityResult{}, fmt.Errorf("storage: continuity mutation run is not owned by the session")
	case err == nil:
	case errors.Is(err, sql.ErrNoRows):
		return storage.ContinuityResult{}, storage.ErrNotFound
	default:
		return storage.ContinuityResult{}, fmt.Errorf("storage: lock continuity run %s: %w", m.RunID, err)
	}

	existing, found, err := postgresFindReceiptTx(ctx, tx, m.SessionID, m.Receipt.Operation, m.Receipt.RequestID)
	if err != nil {
		return storage.ContinuityResult{}, err
	}
	if found {
		if existing.InputHash != m.InputHash || existing.RunID != m.RunID {
			return storage.ContinuityResult{}, storage.ErrConflict
		}
		var replay domain.RunEvent
		var typ string
		err := tx.QueryRowContext(ctx,
			`SELECT run_id, seq, type, created_at, payload_version, payload FROM run_events WHERE run_id = $1 AND seq = $2`,
			m.RunID, int64(existing.EventSeq)).
			Scan((*string)(&replay.RunID), (*int64)(&replay.Seq), &typ, &replay.CreatedAt, &replay.PayloadVersion, &replay.Payload)
		if errors.Is(err, sql.ErrNoRows) {
			return storage.ContinuityResult{}, fmt.Errorf("storage: continuity receipt points at missing event")
		}
		if err != nil {
			return storage.ContinuityResult{}, fmt.Errorf("storage: replay continuity operation: %w", err)
		}
		replay.Type = domain.EventType(typ)
		return storage.ContinuityResult{RunID: m.RunID, Events: []domain.RunEvent{replay}, Receipt: existing, NewlyCommitted: false}, nil
	}

	var closed int
	if err := tx.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM run_events WHERE run_id = $1 AND type IN `+terminalTypes,
		m.RunID).Scan(&closed); err != nil {
		return storage.ContinuityResult{}, fmt.Errorf("storage: check terminal: %w", err)
	}
	if closed > 0 {
		return storage.ContinuityResult{}, storage.ErrRunClosed
	}
	var maxSeq sql.NullInt64
	if err := tx.QueryRowContext(ctx,
		`SELECT MAX(seq) FROM run_events WHERE run_id = $1`, m.RunID).Scan(&maxSeq); err != nil {
		return storage.ContinuityResult{}, fmt.Errorf("storage: read max seq: %w", err)
	}
	event := m.Event
	if err := postgresInsertContinuityEvent(ctx, tx, &event, maxSeq.Int64+1); err != nil {
		return storage.ContinuityResult{}, err
	}
	if err := postgresFault(m.TestFaultHook, "events"); err != nil {
		return storage.ContinuityResult{}, err
	}
	receipt := m.Receipt
	receipt.RunID = m.RunID
	receipt.EventSeq = event.Seq
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO continuity_receipts (session_id, operation, request_id, input_hash, run_id, event_seq) VALUES ($1,$2,$3,$4,$5,$6)`,
		receipt.SessionID, receipt.Operation, receipt.RequestID, receipt.InputHash, receipt.RunID, int64(receipt.EventSeq)); err != nil {
		return storage.ContinuityResult{}, fmt.Errorf("storage: insert continuity receipt: %w", err)
	}
	if err := postgresFault(m.TestFaultHook, "receipt"); err != nil {
		return storage.ContinuityResult{}, err
	}
	if err := tx.Commit(); err != nil {
		return storage.ContinuityResult{}, fmt.Errorf("storage: commit continuity operation: %w", storage.ErrCommitUncertain)
	}
	return storage.ContinuityResult{RunID: m.RunID, Events: []domain.RunEvent{event}, Receipt: receipt, NewlyCommitted: true}, nil
}
