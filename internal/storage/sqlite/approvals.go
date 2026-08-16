package sqlite

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

// CreateApproval inserts one approval row; callers create rows pending and
// settle them through the first-writer-wins lifecycle methods below.
func (b *Backend) CreateApproval(ctx context.Context, a domain.Approval) error {
	if a.CreatedAt == 0 {
		a.CreatedAt = time.Now().UnixMilli()
	}
	riskFindings, err := json.Marshal(a.RiskFindings)
	if err != nil {
		return fmt.Errorf("storage: encode approval %s risk findings: %w", a.ID, err)
	}
	proposalData := a.ProposalData
	if proposalData == nil {
		proposalData = []byte{}
	}
	if _, err := b.db.ExecContext(ctx,
		`INSERT INTO approvals
			 (id, run_id, tool_call_id, decision, expires_at, resume_target, kind, action, target, precondition_hash, preview, risk_findings_json, proposal_data, tool_name, created_at, decided_at, actor, decision_reason, stale_reason)
			 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		a.ID, a.RunID, a.ToolCallID, a.Decision, a.ExpiresAt, a.ResumeTarget, approvalKind(a.Kind),
		a.Action, a.Target, a.PreconditionHash, a.Preview, riskFindings, proposalData, a.ToolName,
		a.CreatedAt, a.DecidedAt, a.Actor, a.DecisionReason, ""); err != nil {
		return fmt.Errorf("storage: create approval %s: %w", a.ID, err)
	}
	return nil
}

// GetApproval loads one approval; absent ids yield storage.ErrNotFound.
func (b *Backend) GetApproval(ctx context.Context, id string) (domain.Approval, error) {
	var a domain.Approval
	var rid string
	var riskFindings, proposalData, decisionReason, staleReason []byte
	err := b.db.QueryRowContext(ctx,
		`SELECT id, run_id, tool_call_id, decision, expires_at, resume_target, kind,
			action, target, precondition_hash, preview, risk_findings_json, proposal_data,
			tool_name, created_at, decided_at, actor, decision_reason, stale_reason
			FROM approvals WHERE id = ?`, id).
		Scan(&a.ID, &rid, &a.ToolCallID, &a.Decision, &a.ExpiresAt, &a.ResumeTarget, &a.Kind,
			&a.Action, &a.Target, &a.PreconditionHash, &a.Preview, &riskFindings, &proposalData,
			&a.ToolName, &a.CreatedAt, &a.DecidedAt, &a.Actor, &decisionReason, &staleReason)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Approval{}, storage.ErrNotFound
	}
	if err != nil {
		return domain.Approval{}, fmt.Errorf("storage: get approval %s: %w", id, err)
	}
	a.RunID = domain.RunID(rid)
	if len(riskFindings) > 0 {
		if err := json.Unmarshal(riskFindings, &a.RiskFindings); err != nil {
			return domain.Approval{}, fmt.Errorf("storage: decode approval %s risk findings: %w", id, err)
		}
	}
	a.ProposalData = append([]byte(nil), proposalData...)
	a.DecisionReason = string(decisionReason)
	_ = staleReason
	return a, nil
}

// ListPendingApprovals returns pending rows, latest expiry first.
func (b *Backend) ListPendingApprovals(ctx context.Context) ([]domain.Approval, error) {
	rows, err := b.db.QueryContext(ctx,
		`SELECT id, run_id, tool_call_id, decision, expires_at, resume_target, kind,
			action, target, precondition_hash, preview, risk_findings_json, proposal_data,
			tool_name, created_at, decided_at, actor, decision_reason, stale_reason
			FROM approvals WHERE decision = ? ORDER BY expires_at DESC, id`, domain.ApprovalPending)
	if err != nil {
		return nil, fmt.Errorf("storage: list pending approvals: %w", err)
	}
	defer func() { _ = rows.Close() }()

	out := []domain.Approval{}
	for rows.Next() {
		var a domain.Approval
		var rid string
		var riskFindings, proposalData, decisionReason, staleReason []byte
		if err := rows.Scan(&a.ID, &rid, &a.ToolCallID, &a.Decision, &a.ExpiresAt, &a.ResumeTarget, &a.Kind,
			&a.Action, &a.Target, &a.PreconditionHash, &a.Preview, &riskFindings, &proposalData,
			&a.ToolName, &a.CreatedAt, &a.DecidedAt, &a.Actor, &decisionReason, &staleReason); err != nil {
			return nil, fmt.Errorf("storage: scan approval: %w", err)
		}
		a.RunID = domain.RunID(rid)
		if len(riskFindings) > 0 {
			if err := json.Unmarshal(riskFindings, &a.RiskFindings); err != nil {
				return nil, fmt.Errorf("storage: decode approval risk findings: %w", err)
			}
		}
		a.ProposalData = append([]byte(nil), proposalData...)
		a.DecisionReason = string(decisionReason)
		_ = staleReason
		out = append(out, a)
	}
	return out, rows.Err()
}

func approvalKind(kind string) string {
	if kind == domain.ApprovalKindChild {
		return domain.ApprovalKindChild
	}
	return domain.ApprovalKindRun
}

// DecideApproval settles a pending row with compatibility metadata.
func (b *Backend) DecideApproval(ctx context.Context, id, decision string) (bool, error) {
	return b.DecideApprovalWithMetadata(ctx, id, decision, "local_user", "")
}

// DecideApprovalWithMetadata records the actor and optional reviewer reason.
func (b *Backend) DecideApprovalWithMetadata(ctx context.Context, id, decision, actor, reason string) (bool, error) {
	res, err := b.db.ExecContext(ctx,
		`UPDATE approvals SET decision = ?, decided_at = ?, actor = ?, decision_reason = ?
		 WHERE id = ? AND decision = ?`,
		decision, time.Now().UnixMilli(), actor, reason, id, domain.ApprovalPending)
	if err != nil {
		return false, fmt.Errorf("storage: decide approval %s: %w", id, err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("storage: decide approval %s: rows affected: %w", id, err)
	}
	return n > 0, nil
}

// ExpireApproval is the system timeout transition.
func (b *Backend) ExpireApproval(ctx context.Context, id, reason string) (bool, error) {
	res, err := b.db.ExecContext(ctx,
		`UPDATE approvals SET decision = ?, decided_at = ?, actor = ?, decision_reason = ?
		 WHERE id = ? AND decision = ?`,
		domain.ApprovalExpired, time.Now().UnixMilli(), "system", reason, id, domain.ApprovalPending)
	if err != nil {
		return false, fmt.Errorf("storage: expire approval %s: %w", id, err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("storage: expire approval %s: rows affected: %w", id, err)
	}
	return n > 0, nil
}

// CancelApproval closes a pending approval when its owning run is explicitly
// cancelled. This prevents a cancelled run from leaving a misleading queue
// item that could later be approved.
func (b *Backend) CancelApproval(ctx context.Context, id, actor, reason string) (bool, error) {
	res, err := b.db.ExecContext(ctx,
		`UPDATE approvals SET decision = ?, decided_at = ?, actor = ?, decision_reason = ?
		 WHERE id = ? AND decision = ?`,
		domain.ApprovalCancelled, time.Now().UnixMilli(), actor, reason, id, domain.ApprovalPending)
	if err != nil {
		return false, fmt.Errorf("storage: cancel approval %s: %w", id, err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("storage: cancel approval %s: rows affected: %w", id, err)
	}
	return n > 0, nil
}

// MarkApprovalStale records a fail-closed precondition mismatch after an
// approval was granted but before the mutation could be applied.
func (b *Backend) MarkApprovalStale(ctx context.Context, id, reason string) (bool, error) {
	res, err := b.db.ExecContext(ctx,
		`UPDATE approvals SET decision = ?, decided_at = ?, actor = ?, decision_reason = ?, stale_reason = ?
		 WHERE id = ? AND decision = ?`,
		domain.ApprovalStale, time.Now().UnixMilli(), "system", reason, reason, id, domain.ApprovalApproved)
	if err != nil {
		return false, fmt.Errorf("storage: mark approval stale %s: %w", id, err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("storage: mark approval stale %s: rows affected: %w", id, err)
	}
	return n > 0, nil
}
