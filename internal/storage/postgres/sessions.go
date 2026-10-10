package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/storage"
)

// CreateSession inserts one session row.
func (b *Backend) CreateSession(ctx context.Context, s domain.Session) error {
	mode, policy := s.EffectiveSandbox()
	if s.UpdatedAt <= 0 {
		s.UpdatedAt = s.CreatedAt
	}
	if s.UpdatedAt <= 0 {
		s.UpdatedAt = time.Now().UnixMilli()
	}
	if _, err := b.db.ExecContext(ctx,
		`INSERT INTO sessions (id, title, created_at, updated_at, sandbox_mode, approval_policy, workspace_path) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		s.ID, s.Title, s.CreatedAt, s.UpdatedAt, string(mode), string(policy), s.WorkspacePath); err != nil {
		return fmt.Errorf("storage: create session %s: %w", s.ID, err)
	}
	return nil
}

// ListSessions returns top-level conversations by durable activity, newest
// first. Addressable child sessions are opened through the child inspector.
func (b *Backend) ListSessions(ctx context.Context) ([]domain.Session, error) {
	rows, err := b.db.QueryContext(ctx,
		`SELECT s.id, s.title, s.created_at, s.updated_at, s.sandbox_mode, s.approval_policy, s.workspace_path
		 FROM sessions s
		 WHERE NOT EXISTS (SELECT 1 FROM child_sessions c WHERE c.child_session_id = s.id)
		 ORDER BY s.updated_at DESC, s.id`)
	if err != nil {
		return nil, fmt.Errorf("storage: list sessions: %w", err)
	}
	defer func() { _ = rows.Close() }()

	out := []domain.Session{}
	for rows.Next() {
		var s domain.Session
		var id string
		if err := rows.Scan(&id, &s.Title, &s.CreatedAt, &s.UpdatedAt, &s.SandboxMode, &s.ApprovalPolicy, &s.WorkspacePath); err != nil {
			return nil, fmt.Errorf("storage: scan session: %w", err)
		}
		s.ID = domain.SessionID(id)
		out = append(out, s)
	}
	return out, rows.Err()
}

// GetSession loads one session; absent ids yield storage.ErrNotFound.
func (b *Backend) GetSession(ctx context.Context, id domain.SessionID) (domain.Session, error) {
	var s domain.Session
	err := b.db.QueryRowContext(ctx,
		`SELECT id, title, created_at, updated_at, sandbox_mode, approval_policy, workspace_path FROM sessions WHERE id = ?`, id).
		Scan((*string)(&s.ID), &s.Title, &s.CreatedAt, &s.UpdatedAt, &s.SandboxMode, &s.ApprovalPolicy, &s.WorkspacePath)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Session{}, storage.ErrNotFound
	}
	if err != nil {
		return domain.Session{}, fmt.Errorf("storage: get session %s: %w", id, err)
	}
	return s, nil
}

// UpdateSessionWorkspace changes an empty conversation's workspace. It locks
// the session row under the same protocol as CreateRun so separate processes
// cannot commit a first run concurrently with a workspace change.
func (b *Backend) UpdateSessionWorkspace(ctx context.Context, id domain.SessionID, path string) error {
	tx, err := b.db.SQL.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("storage: begin session workspace update %s: %w", id, err)
	}
	defer func() { _ = tx.Rollback() }()
	var sessionID string
	if err := tx.QueryRowContext(ctx, `SELECT id FROM sessions WHERE id = $1 FOR UPDATE`, id).Scan(&sessionID); errors.Is(err, sql.ErrNoRows) {
		return storage.ErrNotFound
	} else if err != nil {
		return fmt.Errorf("storage: lock session workspace %s: %w", id, err)
	}
	var started bool
	if err := tx.QueryRowContext(ctx, `SELECT EXISTS (SELECT 1 FROM runs WHERE session_id = $1)`, id).Scan(&started); err != nil {
		return fmt.Errorf("storage: inspect session workspace history %s: %w", id, err)
	}
	if started {
		return storage.ErrConflict
	}
	at := time.Now().UnixMilli()
	res, err := tx.ExecContext(ctx, `UPDATE sessions
		SET workspace_path = $1, updated_at = CASE WHEN updated_at < $2 THEN $3 ELSE updated_at END
		WHERE id = $4`, path, at, at, id)
	if err != nil {
		return fmt.Errorf("storage: update session workspace %s: %w", id, err)
	}
	if n, err := res.RowsAffected(); err != nil {
		return fmt.Errorf("storage: update session workspace rows: %w", err)
	} else if n == 0 {
		return storage.ErrNotFound
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("storage: commit session workspace update %s: %w", id, err)
	}
	return nil
}

// RenameSession updates the title; absent ids yield storage.ErrNotFound.
func (b *Backend) RenameSession(ctx context.Context, id domain.SessionID, title string) error {
	at := time.Now().UnixMilli()
	res, err := b.db.ExecContext(ctx,
		`UPDATE sessions SET title = ?, updated_at = CASE WHEN updated_at < ? THEN ? ELSE updated_at END WHERE id = ?`, title, at, at, id)
	if err != nil {
		return fmt.Errorf("storage: rename session %s: %w", id, err)
	}
	return requireAffected(res, "rename session")
}

// UpdateSandboxPolicy writes the session's sandbox knobs; absent ids yield storage.ErrNotFound.
func (b *Backend) UpdateSandboxPolicy(ctx context.Context, id domain.SessionID, mode domain.SandboxMode, policy domain.ApprovalPolicy) error {
	if !mode.Valid() {
		return fmt.Errorf("storage: invalid sandbox mode %q", mode)
	}
	if !policy.Valid() {
		return fmt.Errorf("storage: invalid approval policy %q", policy)
	}
	at := time.Now().UnixMilli()
	res, err := b.db.ExecContext(ctx,
		`UPDATE sessions SET sandbox_mode = ?, approval_policy = ?, updated_at = CASE WHEN updated_at < ? THEN ? ELSE updated_at END WHERE id = ?`,
		string(mode), string(policy), at, at, id)
	if err != nil {
		return fmt.Errorf("storage: update sandbox policy %s: %w", id, err)
	}
	return requireAffected(res, "update sandbox policy")
}

// TouchSession advances durable activity without allowing an out-of-order
// asynchronous writer to move the timestamp backwards.
func (b *Backend) TouchSession(ctx context.Context, id domain.SessionID, at int64) error {
	if at <= 0 {
		at = time.Now().UnixMilli()
	}
	res, err := b.db.ExecContext(ctx,
		`UPDATE sessions SET updated_at = CASE WHEN updated_at < ? THEN ? ELSE updated_at END WHERE id = ?`, at, at, id)
	if err != nil {
		return fmt.Errorf("storage: touch session %s: %w", id, err)
	}
	return requireAffected(res, "touch session")
}

// DeleteSession removes the session and, in one transaction, its
// messages, runs and journal events. Absent ids yield storage.ErrNotFound.
func (b *Backend) DeleteSession(ctx context.Context, id domain.SessionID) error {
	tx, err := b.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("storage: begin delete session: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	var lockedID string
	if err := tx.QueryRowContext(ctx, `SELECT id FROM sessions WHERE id = ? FOR UPDATE`, id).Scan(&lockedID); errors.Is(err, sql.ErrNoRows) {
		return storage.ErrNotFound
	} else if err != nil {
		return fmt.Errorf("storage: lock session %s for delete: %w", id, err)
	}
	subtree, err := postgresChildSessionTree(ctx, tx, id)
	if err != nil {
		return err
	}
	for i := len(subtree) - 1; i >= 0; i-- {
		sessionID := subtree[i]
		for _, stmt := range []struct{ sql string }{
			{`DELETE FROM session_work_events WHERE session_id = ?`},
			{`DELETE FROM channel_deliveries WHERE session_id = ?`},
			{`DELETE FROM run_events WHERE run_id IN (SELECT id FROM runs WHERE session_id = ?)`},
			{`DELETE FROM run_events WHERE run_id IN (SELECT 'sessctl_queue_' || id FROM runs WHERE session_id = ?)`},
			{`DELETE FROM run_events WHERE run_id IN (SELECT run_id FROM session_compactions WHERE session_id = ?)`},
			{`DELETE FROM run_prompt_snapshots WHERE run_id IN (SELECT id FROM runs WHERE session_id = ?)`},
			{`DELETE FROM tool_operations WHERE run_id IN (SELECT id FROM runs WHERE session_id = ?)`},
			{`DELETE FROM workflow_revisions WHERE parent_session_id = ?`},
			{`DELETE FROM child_message_receipts WHERE child_session_id = ?`},
			{`DELETE FROM child_mailbox_messages WHERE child_session_id = ? OR sender_session_id = ? OR recipient_session_id = ?`},
			{`DELETE FROM child_sessions WHERE child_session_id = ? OR origin_parent_session_id = ?`},
			{`DELETE FROM runs WHERE session_id = ?`},
			{`DELETE FROM message_attachments WHERE message_id IN (SELECT id FROM messages WHERE session_id = ?)`},
			{`DELETE FROM message_file_contexts WHERE message_id IN (SELECT id FROM messages WHERE session_id = ?)`},
			{`DELETE FROM messages WHERE session_id = ?`},
			{`DELETE FROM session_compactions WHERE session_id = ?`},
			{`DELETE FROM session_mask_selections WHERE session_id = ?`},
			{`DELETE FROM session_truncations WHERE session_id = ?`},
			{`DELETE FROM file_versions WHERE session_id = ?`},
			{`DELETE FROM file_reads WHERE session_id = ?`},
			{`DELETE FROM sessions WHERE id = ?`},
		} {
			args := []any{sessionID}
			if stmt.sql == `DELETE FROM child_mailbox_messages WHERE child_session_id = ? OR sender_session_id = ? OR recipient_session_id = ?` {
				args = append(args, sessionID, sessionID)
			} else if stmt.sql == `DELETE FROM child_sessions WHERE child_session_id = ? OR origin_parent_session_id = ?` {
				args = append(args, sessionID)
			}
			if _, err := tx.ExecContext(ctx, stmt.sql, args...); err != nil {
				return fmt.Errorf("storage: delete session %s: %w", sessionID, err)
			}
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("storage: commit delete session: %w", err)
	}
	return nil
}

func postgresChildSessionTree(ctx context.Context, tx *Tx, root domain.SessionID) ([]domain.SessionID, error) {
	queue := []domain.SessionID{root}
	seen := map[domain.SessionID]struct{}{root: {}}
	for index := 0; index < len(queue); index++ {
		if index > 0 {
			var lockedID string
			if err := tx.QueryRowContext(ctx, `SELECT id FROM sessions WHERE id = ? FOR UPDATE`, queue[index]).Scan(&lockedID); errors.Is(err, sql.ErrNoRows) {
				// Keep traversing its binding so corrupt partial rows are cleaned.
			} else if err != nil {
				return nil, fmt.Errorf("storage: lock child session %s for delete: %w", queue[index], err)
			}
		}
		rows, err := tx.QueryContext(ctx, `SELECT child_session_id FROM child_sessions WHERE origin_parent_session_id = ? ORDER BY child_session_id`, queue[index])
		if err != nil {
			return nil, fmt.Errorf("storage: list child sessions for delete: %w", err)
		}
		var children []domain.SessionID
		for rows.Next() {
			var childID string
			if err := rows.Scan(&childID); err != nil {
				_ = rows.Close()
				return nil, fmt.Errorf("storage: scan child session for delete: %w", err)
			}
			children = append(children, domain.SessionID(childID))
		}
		if err := rows.Err(); err != nil {
			_ = rows.Close()
			return nil, fmt.Errorf("storage: iterate child sessions for delete: %w", err)
		}
		_ = rows.Close()
		for _, childID := range children {
			if _, found := seen[childID]; found {
				continue
			}
			seen[childID] = struct{}{}
			queue = append(queue, childID)
		}
	}
	return queue, nil
}

// requireAffected maps a zero-row write to storage.ErrNotFound.
func requireAffected(res sql.Result, op string) error {
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("storage: %s: rows affected: %w", op, err)
	}
	if n == 0 {
		return storage.ErrNotFound
	}
	return nil
}
