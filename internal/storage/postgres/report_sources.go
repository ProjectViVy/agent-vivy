package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/storage"
)

var _ storage.ReportSourceStore = (*Backend)(nil)

func (b *Backend) ListReportSourceSessions(ctx context.Context, scope string) ([]domain.Session, error) {
	var rows *sql.Rows
	var err error
	if scope == "home" || scope == "" {
		rows, err = b.db.QueryContext(ctx,
			`SELECT id, title, created_at, updated_at, sandbox_mode, approval_policy, workspace_path, purpose
			 FROM sessions WHERE COALESCE(purpose, '') = '' ORDER BY created_at, id`)
	} else {
		rows, err = b.db.QueryContext(ctx,
			`SELECT id, title, created_at, updated_at, sandbox_mode, approval_policy, workspace_path, purpose
			 FROM sessions WHERE COALESCE(purpose, '') = '' AND workspace_path = ? ORDER BY created_at, id`, scope)
	}
	if err != nil {
		return nil, fmt.Errorf("storage: list report source sessions: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var out []domain.Session
	for rows.Next() {
		var s domain.Session
		var purpose string
		if err := rows.Scan(&s.ID, &s.Title, &s.CreatedAt, &s.UpdatedAt, &s.SandboxMode, &s.ApprovalPolicy, &s.WorkspacePath, &purpose); err != nil {
			return nil, fmt.Errorf("storage: scan report source session: %w", err)
		}
		s.Purpose = domain.SessionPurpose(purpose)
		out = append(out, s)
	}
	return out, rows.Err()
}

func (b *Backend) ListReportSourceMessages(ctx context.Context, sessionID string, startMs, endMs int64, limit int) ([]storage.ReportSourceRow, error) {
	rows, err := b.db.QueryContext(ctx,
		`SELECT id, session_id, run_id, role, content, created_at FROM messages
		 WHERE session_id = ? AND created_at >= ? AND created_at < ?
		   AND COALESCE(tool_call_id, '') = '' AND COALESCE(tool_name, '') = ''
		   AND COALESCE(exclude_automatic_ingest, 0) = 0
		 ORDER BY created_at, id LIMIT ?`, sessionID, startMs, endMs, limit)
	if err != nil {
		return nil, fmt.Errorf("storage: list report source messages: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var out []storage.ReportSourceRow
	for rows.Next() {
		var r storage.ReportSourceRow
		if err := rows.Scan(&r.MessageID, &r.SessionID, &r.RunID, &r.Role, &r.Content, &r.CreatedAtMs); err != nil {
			return nil, fmt.Errorf("storage: scan report source message: %w", err)
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func (b *Backend) ListReportFeedback(ctx context.Context, scope string, entryIDs []string) ([]storage.ReportFeedbackRow, error) {
	if len(entryIDs) == 0 {
		return nil, nil
	}
	marks := strings.TrimSuffix(strings.Repeat("?,", len(entryIDs)), ",")
	args := make([]any, 0, len(entryIDs)+1)
	args = append(args, scope)
	for _, id := range entryIDs {
		args = append(args, id)
	}
	rows, err := b.db.QueryContext(ctx,
		`SELECT id, entry_id, version, body FROM notebook_comments
		 WHERE scope = ? AND entry_id IN (`+marks+`) AND status = 'active'
		 ORDER BY entry_id, created_at, id`, args...)
	if err != nil {
		return nil, fmt.Errorf("storage: list report feedback: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var out []storage.ReportFeedbackRow
	for rows.Next() {
		var r storage.ReportFeedbackRow
		var body []byte
		if err := rows.Scan(&r.CommentID, &r.EntryID, &r.Version, &body); err != nil {
			return nil, fmt.Errorf("storage: scan report feedback: %w", err)
		}
		r.Body = string(body)
		out = append(out, r)
	}
	return out, rows.Err()
}

func (b *Backend) FindReportEntry(ctx context.Context, scope, seriesID, windowID string) (storage.ReportEntryHead, error) {
	row := b.db.QueryRowContext(ctx,
		`SELECT id, section_id, title, head_revision_id, version, deleted_at FROM notebook_entries
		 WHERE scope = ? AND report_series_id = ? AND report_window_id = ?`, scope, seriesID, windowID)
	return scanReportEntryHead(row)
}

func (b *Backend) GetReportEntryHead(ctx context.Context, scope, entryID string) (storage.ReportEntryHead, error) {
	row := b.db.QueryRowContext(ctx,
		`SELECT id, section_id, title, head_revision_id, version, deleted_at FROM notebook_entries
		 WHERE scope = ? AND id = ?`, scope, entryID)
	return scanReportEntryHead(row)
}

func scanReportEntryHead(row interface{ Scan(dest ...any) error }) (storage.ReportEntryHead, error) {
	var h storage.ReportEntryHead
	var deleted int64
	err := row.Scan(&h.EntryID, &h.SectionID, &h.Title, &h.HeadRevisionID, &h.Version, &deleted)
	if errors.Is(err, sql.ErrNoRows) {
		return storage.ReportEntryHead{}, storage.ErrNotFound
	}
	if err != nil {
		return storage.ReportEntryHead{}, fmt.Errorf("storage: scan report entry head: %w", err)
	}
	h.Deleted = deleted != 0
	return h, nil
}

func (b *Backend) ListReportHumanEdits(ctx context.Context, scope string, entryIDs []string, limit int) ([]storage.ReportFeedbackRow, error) {
	if len(entryIDs) == 0 {
		return nil, nil
	}
	marks := strings.TrimSuffix(strings.Repeat("?,", len(entryIDs)), ",")
	args := make([]any, 0, len(entryIDs)+1)
	args = append(args, scope)
	for _, id := range entryIDs {
		args = append(args, id)
	}
	args = append(args, limit)
	rows, err := b.db.QueryContext(ctx,
		`SELECT r.revision_id, r.entry_id, e.version, r.title FROM notebook_revisions r
		 JOIN notebook_entries e ON e.scope = r.scope AND e.id = r.entry_id AND e.head_revision_id = r.revision_id
		 WHERE r.scope = ? AND r.entry_id IN (`+marks+`) AND r.origin <> 'generated'
		 ORDER BY r.entry_id, r.created_at DESC LIMIT ?`, args...)
	if err != nil {
		return nil, fmt.Errorf("storage: list report human edits: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var out []storage.ReportFeedbackRow
	for rows.Next() {
		var r storage.ReportFeedbackRow
		if err := rows.Scan(&r.CommentID, &r.EntryID, &r.Version, &r.Body); err != nil {
			return nil, fmt.Errorf("storage: scan report human edit: %w", err)
		}
		out = append(out, r)
	}
	return out, rows.Err()
}
