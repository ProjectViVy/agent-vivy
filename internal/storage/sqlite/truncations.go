package sqlite

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
	_, err := b.db.ExecContext(ctx, `
		INSERT INTO session_truncations (session_id, cutoff_message_id, reason, fork_session_id, created_at)
		VALUES (?, ?, ?, ?, ?)`,
		t.SessionID, t.CutoffMessageID, t.Reason, t.ForkSessionID, t.CreatedAt)
	if err != nil {
		return fmt.Errorf("storage: record session truncation: %w", err)
	}
	return nil
}

// LatestSessionTruncation returns the newest marker for the session.
func (b *Backend) LatestSessionTruncation(ctx context.Context, sessionID domain.SessionID) (storage.SessionTruncation, bool, error) {
	var (
		t             storage.SessionTruncation
		sid           string
		forkSessionID string
	)
	err := b.db.QueryRowContext(ctx, `
		SELECT session_id, cutoff_message_id, reason, fork_session_id, created_at
		FROM session_truncations WHERE session_id = ?
		ORDER BY id DESC LIMIT 1`, sessionID).
		Scan(&sid, &t.CutoffMessageID, &t.Reason, &forkSessionID, &t.CreatedAt)
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
