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
	sandboxMode := a.SandboxMode
	if sandboxMode == "" {
		sandboxMode = "workspace_write"
	}
	approvalPolicy := a.ApprovalPolicy
	if approvalPolicy == "" {
		approvalPolicy = "ask"
	}
	timeoutAt := a.TimeoutAt
	if _, err := b.db.ExecContext(ctx,
		`INSERT INTO approvals
			 (id, run_id, tool_call_id, decision, expires_at, resume_target, kind, action, target, precondition_hash, preview, risk_findings_json, proposal_data, tool_name, created_at, decided_at, actor, decision_reason, stale_reason, sandbox_mode, approval_policy, timeout_at)
			 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		a.ID, a.RunID, a.ToolCallID, a.Decision, a.ExpiresAt, a.ResumeTarget, approvalKind(a.Kind),
		a.Action, a.Target, a.PreconditionHash, a.Preview, riskFindings, proposalData, a.ToolName,
		a.CreatedAt, a.DecidedAt, a.Actor, a.DecisionReason, "", sandboxMode, approvalPolicy, timeoutAt); err != nil {
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
			tool_name, created_at, decided_at, actor, decision_reason, stale_reason,
			sandbox_mode, approval_policy, timeout_at
			FROM approvals WHERE id = ?`, id).
		Scan(&a.ID, &rid, &a.ToolCallID, &a.Decision, &a.ExpiresAt, &a.ResumeTarget, &a.Kind,
			&a.Action, &a.Target, &a.PreconditionHash, &a.Preview, &riskFindings, &proposalData,
			&a.ToolName, &a.CreatedAt, &a.DecidedAt, &a.Actor, &decisionReason, &staleReason,
			&a.SandboxMode, &a.ApprovalPolicy, &a.TimeoutAt)
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
			tool_name, created_at, decided_at, actor, decision_reason, stale_reason,
			sandbox_mode, approval_policy, timeout_at
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
			&a.ToolName, &a.CreatedAt, &a.DecidedAt, &a.Actor, &decisionReason, &staleReason,
			&a.SandboxMode, &a.ApprovalPolicy, &a.TimeoutAt); err != nil {
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

// ListExpiredApprovals returns pending approvals that have passed their
// timeout deadline. This is used by the background sweeper to auto-expire
// stale approval requests (D-021).
func (b *Backend) ListExpiredApprovals(ctx context.Context) ([]domain.Approval, error) {
	now := time.Now().UnixMilli()
	rows, err := b.db.QueryContext(ctx,
		`SELECT id, run_id, tool_call_id, decision, expires_at, resume_target, kind,
			action, target, precondition_hash, preview, risk_findings_json, proposal_data,
			tool_name, created_at, decided_at, actor, decision_reason, stale_reason,
			sandbox_mode, approval_policy, timeout_at
			FROM approvals 
			WHERE decision = ? AND timeout_at > 0 AND timeout_at <= ?
			ORDER BY timeout_at ASC`, domain.ApprovalPending, now)
	if err != nil {
		return nil, fmt.Errorf("storage: list expired approvals: %w", err)
	}
	defer func() { _ = rows.Close() }()

	out := []domain.Approval{}
	for rows.Next() {
		var a domain.Approval
		var rid string
		var riskFindings, proposalData, decisionReason, staleReason []byte
		if err := rows.Scan(&a.ID, &rid, &a.ToolCallID, &a.Decision, &a.ExpiresAt, &a.ResumeTarget, &a.Kind,
			&a.Action, &a.Target, &a.PreconditionHash, &a.Preview, &riskFindings, &proposalData,
			&a.ToolName, &a.CreatedAt, &a.DecidedAt, &a.Actor, &decisionReason, &staleReason,
			&a.SandboxMode, &a.ApprovalPolicy, &a.TimeoutAt); err != nil {
			return nil, fmt.Errorf("storage: scan expired approval: %w", err)
		}
		a.RunID = domain.RunID(rid)
		if len(riskFindings) > 0 {
			if err := json.Unmarshal(riskFindings, &a.RiskFindings); err != nil {
				return nil, fmt.Errorf("storage: decode expired approval risk findings: %w", err)
			}
		}
		a.ProposalData = append([]byte(nil), proposalData...)
		a.DecisionReason = string(decisionReason)
		_ = staleReason
		out = append(out, a)
	}
	return out, rows.Err()
}

// SweepExpiredApprovals marks all expired pending approvals as expired and
// returns the count of affected rows. This should be called periodically by
// the background scheduler.
func (b *Backend) SweepExpiredApprovals(ctx context.Context) (int, error) {
	now := time.Now().UnixMilli()
	res, err := b.db.ExecContext(ctx,
		`UPDATE approvals SET decision = ?, decided_at = ?, actor = ?, decision_reason = ?
		 WHERE decision = ? AND timeout_at > 0 AND timeout_at <= ?`,
		domain.ApprovalExpired, now, "system", "approval timed out",
		domain.ApprovalPending, now)
	if err != nil {
		return 0, fmt.Errorf("storage: sweep expired approvals: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("storage: sweep expired approvals: rows affected: %w", err)
	}
	return int(n), nil
}

// CreateApprovalWithTimeout creates an approval row with an explicit timeout.
// The timeout_at field is calculated from the current time plus timeoutSeconds.
func (b *Backend) CreateApprovalWithTimeout(ctx context.Context, a domain.Approval, timeoutSeconds int) error {
	if timeoutSeconds > 0 {
		a.TimeoutAt = time.Now().Add(time.Duration(timeoutSeconds) * time.Second).UnixMilli()
	} else {
		// No timeout means use the existing expires_at field only.
		a.TimeoutAt = 0
	}
	return b.CreateApproval(ctx, a)
}
