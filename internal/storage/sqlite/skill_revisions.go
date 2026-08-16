package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/storage"
)

var _ storage.SkillRevisionStore = (*Backend)(nil)

func (b *Backend) CreateSkillRevision(ctx context.Context, r domain.SkillRevision) error {
	if _, err := b.db.ExecContext(ctx, `
		INSERT INTO skill_revisions
		(id, run_id, skill_name, action, target_path, payload, before_payload, base_hash, content_hash, preview, warnings_json, status, created_at, applied_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		r.ID, r.RunID, r.SkillName, r.Action, r.TargetPath, r.Payload, r.BeforePayload, r.BaseHash,
		r.ContentHash, r.Preview, r.WarningsJSON, r.Status, r.CreatedAt, r.AppliedAt); err != nil {
		return fmt.Errorf("storage: create skill revision %s: %w", r.ID, err)
	}
	return nil
}

func (b *Backend) GetSkillRevision(ctx context.Context, id string) (domain.SkillRevision, error) {
	var r domain.SkillRevision
	var runID, status string
	err := b.db.QueryRowContext(ctx, `
		SELECT id, run_id, skill_name, action, target_path, payload, before_payload, base_hash, content_hash, preview, warnings_json, status, created_at, applied_at
		FROM skill_revisions WHERE id = ?`, id).Scan(
		&r.ID, &runID, &r.SkillName, &r.Action, &r.TargetPath, &r.Payload, &r.BeforePayload, &r.BaseHash,
		&r.ContentHash, &r.Preview, &r.WarningsJSON, &status, &r.CreatedAt, &r.AppliedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.SkillRevision{}, storage.ErrNotFound
	}
	if err != nil {
		return domain.SkillRevision{}, fmt.Errorf("storage: get skill revision %s: %w", id, err)
	}
	r.RunID = domain.RunID(runID)
	r.Status = domain.SkillRevisionStatus(status)
	return r, nil
}

func (b *Backend) ListPendingSkillRevisions(ctx context.Context) ([]domain.SkillRevision, error) {
	rows, err := b.db.QueryContext(ctx, `
		SELECT id, run_id, skill_name, action, target_path, payload, before_payload, base_hash, content_hash, preview, warnings_json, status, created_at, applied_at
		FROM skill_revisions WHERE status = ? ORDER BY created_at DESC, id`, domain.SkillRevisionPending)
	if err != nil {
		return nil, fmt.Errorf("storage: list pending skill revisions: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var out []domain.SkillRevision
	for rows.Next() {
		var r domain.SkillRevision
		var runID, status string
		if err := rows.Scan(&r.ID, &runID, &r.SkillName, &r.Action, &r.TargetPath, &r.Payload, &r.BeforePayload, &r.BaseHash,
			&r.ContentHash, &r.Preview, &r.WarningsJSON, &status, &r.CreatedAt, &r.AppliedAt); err != nil {
			return nil, fmt.Errorf("storage: scan skill revision: %w", err)
		}
		r.RunID = domain.RunID(runID)
		r.Status = domain.SkillRevisionStatus(status)
		out = append(out, r)
	}
	return out, rows.Err()
}

func (b *Backend) SetSkillRevisionStatus(ctx context.Context, id string, status domain.SkillRevisionStatus, appliedAt int64) error {
	res, err := b.db.ExecContext(ctx, `
		UPDATE skill_revisions SET status = ?, applied_at = ?
		WHERE id = ? AND status = ?`, status, appliedAt, id, domain.SkillRevisionPending)
	if err != nil {
		return fmt.Errorf("storage: set skill revision %s status: %w", id, err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("storage: set skill revision %s status rows: %w", id, err)
	}
	if n == 0 {
		return storage.ErrVersionConflict
	}
	return nil
}
