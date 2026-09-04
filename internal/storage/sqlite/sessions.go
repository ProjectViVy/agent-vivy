package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/storage"
)

// CreateSession inserts one session row.
func (b *Backend) CreateSession(ctx context.Context, s domain.Session) error {
	mode, policy := s.EffectiveSandbox()
	if _, err := b.db.ExecContext(ctx,
		`INSERT INTO sessions (id, title, created_at, sandbox_mode, approval_policy) VALUES (?, ?, ?, ?, ?)`,
		s.ID, s.Title, s.CreatedAt, string(mode), string(policy)); err != nil {
		return fmt.Errorf("storage: create session %s: %w", s.ID, err)
	}
	return nil
}

// ListSessions returns all sessions, newest first.
func (b *Backend) ListSessions(ctx context.Context) ([]domain.Session, error) {
	rows, err := b.db.QueryContext(ctx,
		`SELECT id, title, created_at, sandbox_mode, approval_policy FROM sessions ORDER BY created_at DESC, id`)
	if err != nil {
		return nil, fmt.Errorf("storage: list sessions: %w", err)
	}
	defer func() { _ = rows.Close() }()

	out := []domain.Session{}
	for rows.Next() {
		var s domain.Session
		var id string
		if err := rows.Scan(&id, &s.Title, &s.CreatedAt, &s.SandboxMode, &s.ApprovalPolicy); err != nil {
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
		`SELECT id, title, created_at, sandbox_mode, approval_policy FROM sessions WHERE id = ?`, id).
		Scan((*string)(&s.ID), &s.Title, &s.CreatedAt, &s.SandboxMode, &s.ApprovalPolicy)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Session{}, storage.ErrNotFound
	}
	if err != nil {
		return domain.Session{}, fmt.Errorf("storage: get session %s: %w", id, err)
	}
	return s, nil
}

// RenameSession updates the title; absent ids yield storage.ErrNotFound.
func (b *Backend) RenameSession(ctx context.Context, id domain.SessionID, title string) error {
	res, err := b.db.ExecContext(ctx,
		`UPDATE sessions SET title = ? WHERE id = ?`, title, id)
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
	res, err := b.db.ExecContext(ctx,
		`UPDATE sessions SET sandbox_mode = ?, approval_policy = ? WHERE id = ?`,
		string(mode), string(policy), id)
	if err != nil {
		return fmt.Errorf("storage: update sandbox policy %s: %w", id, err)
	}
	return requireAffected(res, "update sandbox policy")
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
		{`DELETE FROM run_events WHERE run_id IN (SELECT id FROM runs WHERE session_id = ?)`},
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
