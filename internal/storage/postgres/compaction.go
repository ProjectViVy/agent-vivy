package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/storage"
)

var _ storage.CompactionStore = (*Backend)(nil)

// SaveSessionCompaction persists one session-level compact record. A repeat
// save for the same (session, created_at, run_id) updates in place.
func (b *Backend) SaveSessionCompaction(ctx context.Context, c storage.SessionCompaction) error {
	_, err := b.db.SQL.ExecContext(ctx, `
		INSERT INTO session_compactions (session_id, run_id, summary, tail_from, dropped_count, created_at)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT(session_id, created_at, run_id) DO UPDATE SET
			summary = EXCLUDED.summary,
			tail_from = EXCLUDED.tail_from,
			dropped_count = EXCLUDED.dropped_count`,
		c.SessionID, c.RunID, []byte(c.Summary), c.TailFrom, c.DroppedCount, c.CreatedAt)
	if err != nil {
		return fmt.Errorf("storage: save session compaction: %w", err)
	}
	return nil
}

// LatestSessionCompaction returns the newest record for the session.
func (b *Backend) LatestSessionCompaction(ctx context.Context, sessionID domain.SessionID) (storage.SessionCompaction, bool, error) {
	var (
		c          storage.SessionCompaction
		sid, runID string
		summary    []byte
	)
	err := b.db.SQL.QueryRowContext(ctx, `
		SELECT session_id, run_id, summary, tail_from, dropped_count, created_at
		FROM session_compactions WHERE session_id = $1
		ORDER BY created_at DESC, run_id DESC LIMIT 1`, sessionID).
		Scan(&sid, &runID, &summary, &c.TailFrom, &c.DroppedCount, &c.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return storage.SessionCompaction{}, false, nil
	}
	if err != nil {
		return storage.SessionCompaction{}, false, fmt.Errorf("storage: latest session compaction: %w", err)
	}
	c.SessionID = domain.SessionID(sid)
	c.RunID = domain.RunID(runID)
	c.Summary = string(summary)
	return c, true, nil
}
