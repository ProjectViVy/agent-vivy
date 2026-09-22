package sqlite

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

// ListSessions returns all sessions by durable activity, newest first.
func (b *Backend) ListSessions(ctx context.Context) ([]domain.Session, error) {
	rows, err := b.db.QueryContext(ctx,
		`SELECT id, title, created_at, updated_at, sandbox_mode, approval_policy, workspace_path FROM sessions ORDER BY updated_at DESC, id`)
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

// UpdateSessionWorkspace changes an empty conversation's workspace. The
// NOT EXISTS guard closes the race with creation of the first durable run.
func (b *Backend) UpdateSessionWorkspace(ctx context.Context, id domain.SessionID, path string) error {
	at := time.Now().UnixMilli()
	res, err := b.db.ExecContext(ctx, `UPDATE sessions
		SET workspace_path = ?, updated_at = CASE WHEN updated_at < ? THEN ? ELSE updated_at END
		WHERE id = ? AND NOT EXISTS (SELECT 1 FROM runs WHERE session_id = ?)`, path, at, at, id, id)
	if err != nil {
		return fmt.Errorf("storage: update session workspace %s: %w", id, err)
	}
	if n, err := res.RowsAffected(); err != nil {
		return fmt.Errorf("storage: update session workspace rows: %w", err)
	} else if n > 0 {
		return nil
	}
	if _, err := b.GetSession(ctx, id); errors.Is(err, storage.ErrNotFound) {
		return storage.ErrNotFound
	} else if err != nil {
		return err
	}
	return storage.ErrConflict
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

	var n int
	if err := tx.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM sessions WHERE id = ?`, id).Scan(&n); err != nil {
		return fmt.Errorf("storage: check session %s: %w", id, err)
	}
	if n == 0 {
		return storage.ErrNotFound
	}

	for _, stmt := range []struct{ sql string }{
		{`DELETE FROM session_work_events WHERE session_id = ?`},
		{`DELETE FROM run_events WHERE run_id IN (SELECT id FROM runs WHERE session_id = ?)`},
		{`DELETE FROM run_events WHERE run_id IN (SELECT run_id FROM session_compactions WHERE session_id = ?)`},
		{`DELETE FROM runs WHERE session_id = ?`},
		{`DELETE FROM message_attachments WHERE message_id IN (SELECT id FROM messages WHERE session_id = ?)`},
		{`DELETE FROM message_file_contexts WHERE message_id IN (SELECT id FROM messages WHERE session_id = ?)`},
		{`DELETE FROM messages WHERE session_id = ?`},
		{`DELETE FROM session_compactions WHERE session_id = ?`},
		{`DELETE FROM session_truncations WHERE session_id = ?`},
		{`DELETE FROM file_versions WHERE session_id = ?`},
		{`DELETE FROM file_reads WHERE session_id = ?`},
		{`DELETE FROM sessions WHERE id = ?`},
	} {
		if _, err := tx.ExecContext(ctx, stmt.sql, id); err != nil {
			return fmt.Errorf("storage: delete session %s: %w", id, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("storage: commit delete session: %w", err)
	}
	return nil
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
