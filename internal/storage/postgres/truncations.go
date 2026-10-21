package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/storage"
)

var _ storage.TruncationStore = (*Backend)(nil)

// RecordSessionTruncation appends one cutoff marker. Rows are never
// updated: a new rewind is just a new row, and the newest one wins.
func (b *Backend) RecordSessionTruncation(ctx context.Context, t storage.SessionTruncation) error {
	tx, err := b.db.SQL.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("storage: begin session truncation: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	if err := postgresInsertMarker(ctx, tx, t); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("storage: commit session truncation: %w", err)
	}
	return nil
}

// LatestSessionTruncation returns the newest marker of any reason for the
// session (audit reads; may be a fork provenance anchor).
func (b *Backend) LatestSessionTruncation(ctx context.Context, sessionID domain.SessionID) (storage.SessionTruncation, bool, error) {
	var (
		t             storage.SessionTruncation
		sid           string
		forkSessionID string
	)
	err := b.db.SQL.QueryRowContext(ctx, `
		SELECT session_id, run_id, cutoff_message_id, tail_message_id, work_seq, reason, fork_session_id, created_at
		FROM session_truncations WHERE session_id = $1
		ORDER BY id DESC LIMIT 1`, sessionID).
		Scan(&sid, &t.RunID, &t.CutoffMessageID, &t.TailMessageID, &t.WorkSeq, &t.Reason, &forkSessionID, &t.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return storage.SessionTruncation{}, false, nil
	}
	if err != nil {
		return storage.SessionTruncation{}, false, fmt.Errorf("storage: latest session truncation: %w", err)
	}
	t.SessionID = domain.SessionID(sid)
	t.ForkSessionID = forkSessionID
	return t, true, nil
}

// ListViewTruncations returns every view-controlling marker (rewind/edit)
// in insertion order; the view folds their union of closed ranges.
func (b *Backend) ListViewTruncations(ctx context.Context, sessionID domain.SessionID) ([]storage.SessionTruncation, error) {
	rows, err := b.db.SQL.QueryContext(ctx, `
		SELECT session_id, run_id, cutoff_message_id, tail_message_id, work_seq, reason, fork_session_id, created_at
		FROM session_truncations
		WHERE session_id = $1 AND reason IN ($2, $3)
		ORDER BY id ASC`, sessionID, storage.TruncationRewind, storage.TruncationEdit)
	if err != nil {
		return nil, fmt.Errorf("storage: list view truncations: %w", err)
	}
	defer rows.Close()
	out := []storage.SessionTruncation{}
	for rows.Next() {
		var (
			t             storage.SessionTruncation
			sid           string
			forkSessionID string
		)
		if err := rows.Scan(&sid, &t.RunID, &t.CutoffMessageID, &t.TailMessageID, &t.WorkSeq, &t.Reason, &forkSessionID, &t.CreatedAt); err != nil {
			return nil, fmt.Errorf("storage: scan view truncation: %w", err)
		}
		t.SessionID = domain.SessionID(sid)
		t.ForkSessionID = forkSessionID
		out = append(out, t)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("storage: list view truncations: %w", err)
	}
	return out, nil
}

// ListSessionForkLinks returns every fork/forked-from provenance marker
// across all sessions in insertion order — the session-tree edge set.
func (b *Backend) ListSessionForkLinks(ctx context.Context) ([]storage.SessionTruncation, error) {
	rows, err := b.db.SQL.QueryContext(ctx, `
		SELECT session_id, run_id, cutoff_message_id, tail_message_id, work_seq, reason, fork_session_id, created_at
		FROM session_truncations
		WHERE reason IN ($1, $2)
		ORDER BY id ASC`, storage.TruncationFork, storage.TruncationForkedFrom)
	if err != nil {
		return nil, fmt.Errorf("storage: list session fork links: %w", err)
	}
	defer rows.Close()
	out := []storage.SessionTruncation{}
	for rows.Next() {
		var (
			t             storage.SessionTruncation
			sid           string
			forkSessionID string
		)
		if err := rows.Scan(&sid, &t.RunID, &t.CutoffMessageID, &t.TailMessageID, &t.WorkSeq, &t.Reason, &forkSessionID, &t.CreatedAt); err != nil {
			return nil, fmt.Errorf("storage: scan fork link: %w", err)
		}
		t.SessionID = domain.SessionID(sid)
		t.ForkSessionID = forkSessionID
		out = append(out, t)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("storage: list session fork links: %w", err)
	}
	return out, nil
}
