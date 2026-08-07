package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/storage"
)

// CreateApproval inserts one approval row; callers create rows pending
// and settle them via DecideApproval (FR-6).
func (b *Backend) CreateApproval(ctx context.Context, a domain.Approval) error {
	if _, err := b.db.ExecContext(ctx,
		`INSERT INTO approvals (id, run_id, tool_call_id, decision, expires_at, resume_target)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		a.ID, a.RunID, a.ToolCallID, a.Decision, a.ExpiresAt, a.ResumeTarget); err != nil {
		return fmt.Errorf("storage: create approval %s: %w", a.ID, err)
	}
	return nil
}

// GetApproval loads one approval; absent ids yield storage.ErrNotFound.
func (b *Backend) GetApproval(ctx context.Context, id string) (domain.Approval, error) {
	var a domain.Approval
	var rid string
	err := b.db.QueryRowContext(ctx,
		`SELECT id, run_id, tool_call_id, decision, expires_at, resume_target
		 FROM approvals WHERE id = ?`, id).
		Scan(&a.ID, &rid, &a.ToolCallID, &a.Decision, &a.ExpiresAt, &a.ResumeTarget)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Approval{}, storage.ErrNotFound
	}
	if err != nil {
		return domain.Approval{}, fmt.Errorf("storage: get approval %s: %w", id, err)
	}
	a.RunID = domain.RunID(rid)
	return a, nil
}

// ListPendingApprovals returns pending rows, latest expiry first (the UI
// judges expiry itself off expires_at; D-009 stays server-enforced).
func (b *Backend) ListPendingApprovals(ctx context.Context) ([]domain.Approval, error) {
	rows, err := b.db.QueryContext(ctx,
		`SELECT id, run_id, tool_call_id, decision, expires_at, resume_target
		 FROM approvals WHERE decision = ? ORDER BY expires_at DESC, id`,
		domain.ApprovalPending)
	if err != nil {
		return nil, fmt.Errorf("storage: list pending approvals: %w", err)
	}
	defer func() { _ = rows.Close() }()

	out := []domain.Approval{}
	for rows.Next() {
		var a domain.Approval
		var rid string
		if err := rows.Scan(&a.ID, &rid, &a.ToolCallID, &a.Decision, &a.ExpiresAt, &a.ResumeTarget); err != nil {
			return nil, fmt.Errorf("storage: scan approval: %w", err)
		}
		a.RunID = domain.RunID(rid)
		out = append(out, a)
	}
	return out, rows.Err()
}

// DecideApproval settles a pending row; the pending-guard in the WHERE
// keeps concurrent decisions first-writer-wins. decided=false means no
// pending row matched.
func (b *Backend) DecideApproval(ctx context.Context, id, decision string) (bool, error) {
	res, err := b.db.ExecContext(ctx,
		`UPDATE approvals SET decision = ? WHERE id = ? AND decision = ?`,
		decision, id, domain.ApprovalPending)
	if err != nil {
		return false, fmt.Errorf("storage: decide approval %s: %w", id, err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("storage: decide approval %s: rows affected: %w", id, err)
	}
	return n > 0, nil
}
