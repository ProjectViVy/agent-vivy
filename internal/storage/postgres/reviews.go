package postgres

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/storage"
)

// ListReviews returns the unified approval/question read model. The query is
// intentionally built from the durable interaction tables; live runtime
// maps are not a source of truth for Review Center.
func (b *Backend) ListReviews(ctx context.Context, filter storage.ReviewFilter) ([]domain.ReviewItem, error) {
	limit := filter.Limit
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	items := make([]domain.ReviewItem, 0, limit)
	if filter.Kind == "" || filter.Kind == domain.ReviewKindApproval {
		approvals, err := b.listApprovalReviews(ctx, filter, limit)
		if err != nil {
			return nil, err
		}
		items = append(items, approvals...)
	}
	if (filter.Kind == "" || filter.Kind == domain.ReviewKindQuestion) && len(items) < limit {
		questions, err := b.listQuestionReviews(ctx, filter, limit-len(items))
		if err != nil {
			return nil, err
		}
		items = append(items, questions...)
	}
	// Both subqueries are independently ordered. A final stable sort gives
	// Review Center one deterministic queue across interaction kinds.
	sortReviews(items)
	if len(items) > limit {
		items = items[:limit]
	}
	return items, nil
}

func (b *Backend) GetReview(ctx context.Context, id string) (domain.ReviewItem, error) {
	kind := storage.ReviewFilter{Limit: 500}
	if strings.HasPrefix(id, "apr_") {
		kind.Kind = domain.ReviewKindApproval
	} else if strings.HasPrefix(id, "que_") {
		kind.Kind = domain.ReviewKindQuestion
	}
	items, err := b.ListReviews(ctx, kind)
	if err != nil {
		return domain.ReviewItem{}, err
	}
	for _, item := range items {
		if item.ID == id {
			return item, nil
		}
	}
	return domain.ReviewItem{}, storage.ErrNotFound
}

func (b *Backend) listApprovalReviews(ctx context.Context, filter storage.ReviewFilter, limit int) ([]domain.ReviewItem, error) {
	query := `SELECT a.id, a.run_id, a.tool_call_id, a.decision, a.expires_at,
		 a.created_at, a.decided_at, a.actor, a.decision_reason, a.stale_reason,
		 a.tool_name, a.action, a.target, a.precondition_hash, a.preview,
		 a.risk_findings_json, r.session_id, s.title
		 FROM approvals a JOIN runs r ON r.id = a.run_id JOIN sessions s ON s.id = r.session_id`
	where, args := reviewWhere(filter, "approval", "a.decision", "a.expires_at", "r.session_id")
	query += where + ` ORDER BY CASE WHEN a.decision = 'pending' THEN 0 ELSE 1 END,
		 COALESCE(a.created_at, a.expires_at) DESC, a.id LIMIT ?`
	args = append(args, limit)
	rows, err := b.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("storage: list approval reviews: %w", err)
	}
	defer func() { _ = rows.Close() }()
	items := make([]domain.ReviewItem, 0, limit)
	for rows.Next() {
		var item domain.ReviewItem
		var rid, sessionID, title, decision string
		var reason, staleReason, risk []byte
		if err := rows.Scan(&item.ID, &rid, &item.ToolCallID, &decision, &item.ExpiresAt,
			&item.CreatedAt, &item.DecidedAt, &item.Actor, &reason, &staleReason,
			&item.ToolName, &item.Action, &item.Target, &item.PreconditionHash, &item.Preview,
			&risk, &sessionID, &title); err != nil {
			return nil, fmt.Errorf("storage: scan approval review: %w", err)
		}
		item.Kind = domain.ReviewKindApproval
		item.Status = approvalReviewStatus(decision, item.ExpiresAt)
		item.RunID = domain.RunID(rid)
		item.SessionID = domain.SessionID(sessionID)
		item.SessionTitle = title
		item.DecisionReason = string(reason)
		item.StaleReason = string(staleReason)
		if len(risk) > 0 && string(risk) != "null" {
			if err := json.Unmarshal(risk, &item.RiskFindings); err != nil {
				return nil, fmt.Errorf("storage: decode approval review risk: %w", err)
			}
		}
		classifyReview(&item)
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	// SQLite is intentionally configured with one connection. Close the
	// listing cursor before querying each durable approval event for its
	// redacted argument projection.
	if err := rows.Close(); err != nil {
		return nil, fmt.Errorf("storage: close approval reviews: %w", err)
	}
	for i := range items {
		items[i].Arguments = b.reviewArguments(ctx, items[i].RunID, items[i].ID, items[i].ToolCallID)
	}
	return items, nil
}

func (b *Backend) listQuestionReviews(ctx context.Context, filter storage.ReviewFilter, limit int) ([]domain.ReviewItem, error) {
	query := `SELECT q.id, q.run_id, q.tool_call_id, q.prompt, q.status, q.expires_at,
		 q.created_at, q.answered_at, q.actor, q.decision_reason, r.session_id, s.title
		 FROM questions q JOIN runs r ON r.id = q.run_id JOIN sessions s ON s.id = r.session_id`
	where, args := reviewWhere(filter, "question", "q.status", "q.expires_at", "r.session_id")
	query += where + ` ORDER BY CASE WHEN q.status = 'pending' THEN 0 ELSE 1 END,
		 COALESCE(q.created_at, q.expires_at) DESC, q.id LIMIT ?`
	args = append(args, limit)
	rows, err := b.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("storage: list question reviews: %w", err)
	}
	defer func() { _ = rows.Close() }()
	items := make([]domain.ReviewItem, 0, limit)
	for rows.Next() {
		var item domain.ReviewItem
		var rid, sessionID, title, status string
		var reason []byte
		if err := rows.Scan(&item.ID, &rid, &item.ToolCallID, &item.Prompt, &status, &item.ExpiresAt,
			&item.CreatedAt, &item.DecidedAt, &item.Actor, &reason, &sessionID, &title); err != nil {
			return nil, fmt.Errorf("storage: scan question review: %w", err)
		}
		item.Kind = domain.ReviewKindQuestion
		item.Status = questionReviewStatus(status, item.ExpiresAt)
		item.RunID = domain.RunID(rid)
		item.SessionID = domain.SessionID(sessionID)
		item.SessionTitle = title
		item.DecisionReason = string(reason)
		item.ToolName = "ask_user"
		item.Source = "user_input"
		item.Effect = "input"
		item.Reversibility = "none"
		item.Scope = "current run"
		item.Trust = "model requested"
		items = append(items, item)
	}
	return items, rows.Err()
}

func reviewWhere(filter storage.ReviewFilter, kind, statusColumn, expiryColumn, sessionColumn string) (string, []any) {
	parts := make([]string, 0, 2)
	args := make([]any, 0, 2)
	if filter.Status != "" {
		status := string(filter.Status)
		if filter.Status == domain.ReviewPending {
			parts = append(parts, statusColumn+" = ? AND "+expiryColumn+" > ?")
			args = append(args, domain.ApprovalPending, time.Now().UnixMilli())
		} else if filter.Status == domain.ReviewExpired {
			parts = append(parts, statusColumn+" = 'expired'")
		} else {
			status = reviewStorageStatus(filter.Status)
			parts = append(parts, statusColumn+" = ?")
			args = append(args, status)
		}
	}
	if filter.SessionID != "" {
		parts = append(parts, sessionColumn+" = ?")
		args = append(args, string(filter.SessionID))
	}
	if len(parts) == 0 {
		return "", args
	}
	return " WHERE " + strings.Join(parts, " AND "), args
}

func reviewStorageStatus(status domain.ReviewStatus) string {
	switch status {
	case domain.ReviewApproved:
		return domain.ApprovalApproved
	case domain.ReviewDenied:
		return domain.ApprovalDenied
	case domain.ReviewAnswered:
		return string(domain.QuestionAnswered)
	case domain.ReviewCancelled:
		return string(domain.QuestionCancelled)
	case domain.ReviewStale:
		return domain.ApprovalStale
	default:
		return string(status)
	}
}

func approvalReviewStatus(status string, expiresAt int64) domain.ReviewStatus {
	if status == domain.ApprovalPending && expiresAt > 0 && expiresAt <= time.Now().UnixMilli() {
		return domain.ReviewExpired
	}
	if status == domain.ApprovalExpired {
		return domain.ReviewExpired
	}
	if status == domain.ApprovalStale {
		return domain.ReviewStale
	}
	return domain.ReviewStatus(status)
}

func questionReviewStatus(status string, expiresAt int64) domain.ReviewStatus {
	if status == string(domain.QuestionPending) && expiresAt > 0 && expiresAt <= time.Now().UnixMilli() {
		return domain.ReviewExpired
	}
	if status == string(domain.QuestionExpired) {
		return domain.ReviewExpired
	}
	return domain.ReviewStatus(status)
}

func classifyReview(item *domain.ReviewItem) {
	item.Source = "tool"
	item.Effect = "external side effect"
	item.Reversibility = "unknown"
	item.Scope = "current run"
	item.Trust = "policy gated"
	if strings.Contains(item.Action, "file") || strings.Contains(item.Action, "patch") || strings.HasPrefix(item.Action, "skill_") {
		item.Source = "workspace"
		item.Effect = "filesystem mutation"
		item.Reversibility = "precondition protected"
		item.Scope = "run workspace"
	}
	if strings.Contains(item.Action, "http") || strings.Contains(item.Action, "mcp") {
		item.Source = "network"
		item.Scope = "configured endpoint"
	}
}

// reviewArguments recovers the original approval event's args, then applies
// key/value redaction before returning them to RPC/UI. If the event is absent
// (old rows), an empty object is safer than returning ProposalData.
func (b *Backend) reviewArguments(ctx context.Context, runID domain.RunID, reviewID, toolCallID string) json.RawMessage {
	rows, err := b.db.QueryContext(ctx,
		`SELECT payload FROM run_events WHERE run_id = ? AND type = ? ORDER BY seq DESC LIMIT 32`,
		runID, domain.EventToolApprovalRequired)
	if err != nil {
		return json.RawMessage(`{}`)
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var raw []byte
		if rows.Scan(&raw) != nil {
			continue
		}
		var payload map[string]any
		if json.Unmarshal(raw, &payload) != nil {
			continue
		}
		if payload["approval_id"] != reviewID && payload["tool_call_id"] != toolCallID {
			continue
		}
		args, ok := payload["args"]
		if !ok {
			return json.RawMessage(`{}`)
		}
		redacted, err := json.Marshal(redactReviewValue(args, ""))
		if err == nil {
			return redacted
		}
	}
	return json.RawMessage(`{}`)
}

func redactReviewValue(value any, key string) any {
	if m, ok := value.(map[string]any); ok {
		out := make(map[string]any, len(m))
		for k, v := range m {
			lower := strings.ToLower(k)
			normalized := strings.NewReplacer("-", "_", " ", "_").Replace(lower)
			if strings.Contains(normalized, "password") || strings.Contains(normalized, "secret") || strings.Contains(normalized, "token") || strings.Contains(normalized, "api_key") || strings.Contains(normalized, "apikey") || strings.Contains(normalized, "authorization") || strings.Contains(normalized, "credential") || strings.Contains(normalized, "private_key") {
				out[k] = "[REDACTED]"
				continue
			}
			out[k] = redactReviewValue(v, k)
		}
		return out
	}
	if values, ok := value.([]any); ok {
		out := make([]any, len(values))
		for i, v := range values {
			out[i] = redactReviewValue(v, key)
		}
		return out
	}
	if text, ok := value.(string); ok && len(text) > 4096 {
		return text[:4096] + "…"
	}
	return value
}

func sortReviews(items []domain.ReviewItem) {
	for i := 1; i < len(items); i++ {
		current := items[i]
		j := i - 1
		for j >= 0 && reviewBefore(current, items[j]) {
			items[j+1] = items[j]
			j--
		}
		items[j+1] = current
	}
}

func reviewBefore(a, b domain.ReviewItem) bool {
	if a.Status == domain.ReviewPending && b.Status != domain.ReviewPending {
		return true
	}
	if a.Status != domain.ReviewPending && b.Status == domain.ReviewPending {
		return false
	}
	if a.CreatedAt != b.CreatedAt {
		return a.CreatedAt > b.CreatedAt
	}
	return a.ID < b.ID
}
