package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/storage"
)

var _ storage.CompactionStore = (*Backend)(nil)

// SaveSessionCompaction persists one session-level compact record. The
// primary key (session_id, created_at, run_id) allows one summary per run
// while a repeat call in the same run becomes a no-op replace.
func (b *Backend) SaveSessionCompaction(ctx context.Context, c storage.SessionCompaction) error {
	_, err := b.db.ExecContext(ctx, `
		INSERT INTO session_compactions (session_id, run_id, summary, tail_from, dropped_count, created_at)
		VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT(session_id, created_at, run_id) DO UPDATE SET
			summary = excluded.summary,
			tail_from = excluded.tail_from,
			dropped_count = excluded.dropped_count`,
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
	err := b.db.QueryRowContext(ctx, `
		SELECT session_id, run_id, summary, tail_from, dropped_count, created_at
		FROM session_compactions WHERE session_id = ?
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

// ListSessionCompactions returns the session's records newest first.
func (b *Backend) ListSessionCompactions(ctx context.Context, sessionID domain.SessionID, limit int) ([]storage.SessionCompaction, error) {
	if limit <= 0 {
		return nil, nil
	}
	rows, err := b.db.QueryContext(ctx, `
		SELECT session_id, run_id, summary, tail_from, dropped_count, created_at
		FROM session_compactions WHERE session_id = ?
		ORDER BY created_at DESC, run_id DESC LIMIT ?`, sessionID, limit)
	if err != nil {
		return nil, fmt.Errorf("storage: list session compactions: %w", err)
	}
	defer rows.Close()
	var out []storage.SessionCompaction
	for rows.Next() {
		var (
			c          storage.SessionCompaction
			sid, runID string
			summary    []byte
		)
		if err := rows.Scan(&sid, &runID, &summary, &c.TailFrom, &c.DroppedCount, &c.CreatedAt); err != nil {
			return nil, fmt.Errorf("storage: list session compactions: %w", err)
		}
		c.SessionID = domain.SessionID(sid)
		c.RunID = domain.RunID(runID)
		c.Summary = string(summary)
		out = append(out, c)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("storage: list session compactions: %w", err)
	}
	return out, nil
}
