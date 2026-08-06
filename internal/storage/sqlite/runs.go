package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/storage"
)

const activeStatuses = `('accepted','queued','active')`

// CreateRun inserts one run row (status accepted on creation, FR-4).
func (b *Backend) CreateRun(ctx context.Context, r domain.Run) error {
	if _, err := b.db.ExecContext(ctx,
		`INSERT INTO runs (id, session_id, status, created_at) VALUES (?, ?, ?, ?)`,
		r.ID, r.SessionID, string(r.Status), r.CreatedAt); err != nil {
		return fmt.Errorf("storage: create run %s: %w", r.ID, err)
	}
	return nil
}

// GetRun loads one run; absent ids yield storage.ErrNotFound.
func (b *Backend) GetRun(ctx context.Context, id domain.RunID) (domain.Run, error) {
	var r domain.Run
	var rid, sid, status string
	err := b.db.QueryRowContext(ctx,
		`SELECT id, session_id, status, created_at FROM runs WHERE id = ?`, id).
		Scan(&rid, &sid, &status, &r.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Run{}, storage.ErrNotFound
	}
	if err != nil {
		return domain.Run{}, fmt.Errorf("storage: get run %s: %w", id, err)
	}
	r.ID = domain.RunID(rid)
	r.SessionID = domain.SessionID(sid)
	r.Status = domain.RunStatus(status)
	return r, nil
}

// SetRunStatus persists a lifecycle transition; absent ids yield
// storage.ErrNotFound. Transition validity is the domain state machine's
// job, not the store's.
func (b *Backend) SetRunStatus(ctx context.Context, id domain.RunID, status domain.RunStatus) error {
	res, err := b.db.ExecContext(ctx,
		`UPDATE runs SET status = ? WHERE id = ?`, string(status), id)
	if err != nil {
		return fmt.Errorf("storage: set run %s status: %w", id, err)
	}
	return requireAffected(res, "set run status")
}

// ListActiveRuns enumerates non-terminal runs in creation order (restart
// recovery input, E2).
func (b *Backend) ListActiveRuns(ctx context.Context) ([]domain.Run, error) {
	rows, err := b.db.QueryContext(ctx,
		`SELECT id, session_id, status, created_at FROM runs
		 WHERE status IN `+activeStatuses+` ORDER BY created_at, id`)
	if err != nil {
		return nil, fmt.Errorf("storage: list active runs: %w", err)
	}
	defer func() { _ = rows.Close() }()

	out := []domain.Run{}
	for rows.Next() {
		var r domain.Run
		var rid, sid, status string
		if err := rows.Scan(&rid, &sid, &status, &r.CreatedAt); err != nil {
			return nil, fmt.Errorf("storage: scan run: %w", err)
		}
		r.ID = domain.RunID(rid)
		r.SessionID = domain.SessionID(sid)
		r.Status = domain.RunStatus(status)
		out = append(out, r)
	}
	return out, rows.Err()
}
