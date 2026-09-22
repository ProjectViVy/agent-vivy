package postgres

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
	kind := r.Kind
	if !kind.Valid() {
		kind = domain.RunKindPrimary
	}
	rootID := r.RootID
	if rootID == "" {
		rootID = r.ID
	}
	tx, err := b.db.SQL.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("storage: begin create run %s: %w", r.ID, err)
	}
	defer func() { _ = tx.Rollback() }()
	var sessionID string
	if err := tx.QueryRowContext(ctx, `SELECT id FROM sessions WHERE id = $1 FOR UPDATE`, r.SessionID).Scan(&sessionID); errors.Is(err, sql.ErrNoRows) {
		return storage.ErrNotFound
	} else if err != nil {
		return fmt.Errorf("storage: lock session for run %s: %w", r.ID, err)
	}
	if kind == domain.RunKindPrimary && (r.Status == domain.RunAccepted || r.Status == domain.RunQueued || r.Status == domain.RunActive) {
		var active int
		if err := tx.QueryRowContext(ctx,
			`SELECT COUNT(*) FROM runs WHERE session_id = $1 AND kind = $2 AND status IN `+activeStatuses,
			r.SessionID, string(domain.RunKindPrimary)).Scan(&active); err != nil {
			return fmt.Errorf("storage: inspect active run for %s: %w", r.ID, err)
		}
		if active != 0 {
			return storage.ErrWorkRunConflict
		}
	}
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO runs (id, session_id, status, created_at, kind, parent_run_id, root_run_id, depth)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
		r.ID, r.SessionID, string(r.Status), r.CreatedAt, string(kind), r.ParentID, rootID, r.Depth); err != nil {
		return fmt.Errorf("storage: create run %s: %w", r.ID, err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("storage: commit create run %s: %w", r.ID, err)
	}
	return nil
}

// CommitPrimaryRun persists the user message, active primary run and
// run.started event in one transaction. The session row lock serializes
// backend handles before the active-run check.
func (b *Backend) CommitPrimaryRun(ctx context.Context, admission storage.PrimaryRunCommit) (domain.RunEvent, error) {
	if err := storage.ValidatePrimaryRunCommit(admission); err != nil {
		return domain.RunEvent{}, err
	}
	tx, err := b.db.SQL.BeginTx(ctx, nil)
	if err != nil {
		return domain.RunEvent{}, fmt.Errorf("storage: begin primary run admission: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	var sessionID string
	if err := tx.QueryRowContext(ctx, "SELECT id FROM sessions WHERE id = $1 FOR UPDATE", admission.Message.SessionID).Scan(&sessionID); errors.Is(err, sql.ErrNoRows) {
		return domain.RunEvent{}, storage.ErrNotFound
	} else if err != nil {
		return domain.RunEvent{}, fmt.Errorf("storage: lock primary session: %w", err)
	}

	var active int
	if err := tx.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM runs WHERE session_id = $1 AND kind = $2 AND status IN "+activeStatuses,
		admission.Run.SessionID, string(domain.RunKindPrimary)).Scan(&active); err != nil {
		return domain.RunEvent{}, fmt.Errorf("storage: inspect active primary run: %w", err)
	}
	if active != 0 {
		return domain.RunEvent{}, storage.ErrWorkRunConflict
	}

	message := admission.Message
	if _, err := tx.ExecContext(ctx,
		"INSERT INTO messages (id, session_id, run_id, role, created_at, content, tool_call_id, tool_name, tool_args, source, channel, chat_id, channel_message_id) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)",
		message.ID, message.SessionID, message.RunID, string(message.Role), message.CreatedAt, message.Content,
		message.ToolCallID, message.ToolName, toolArgsBlob(message.ToolArgs),
		message.Source, message.Channel, message.ChatID, message.ChannelMessageID); err != nil {
		return domain.RunEvent{}, fmt.Errorf("storage: append primary message: %w", err)
	}
	for position, attachment := range message.Attachments {
		if _, err := tx.ExecContext(ctx,
			"INSERT INTO message_attachments (message_id, position, name, mime_type, data) VALUES ($1, $2, $3, $4, $5)",
			message.ID, position, attachment.Name, attachment.MimeType, attachment.Data); err != nil {
			return domain.RunEvent{}, fmt.Errorf("storage: append primary attachment: %w", err)
		}
	}
	for position, file := range message.FileContexts {
		if _, err := tx.ExecContext(ctx,
			"INSERT INTO message_file_contexts (message_id, position, path, name, size, content) VALUES ($1, $2, $3, $4, $5, $6)",
			message.ID, position, file.Path, file.Name, file.Size, file.Content); err != nil {
			return domain.RunEvent{}, fmt.Errorf("storage: append primary file context: %w", err)
		}
	}
	at := messageActivityAt(message.CreatedAt)
	if _, err := tx.ExecContext(ctx,
		"UPDATE sessions SET updated_at = CASE WHEN updated_at < $1 THEN $2 ELSE updated_at END WHERE id = $3",
		at, at, message.SessionID); err != nil {
		return domain.RunEvent{}, fmt.Errorf("storage: touch primary session: %w", err)
	}

	run := admission.Run
	kind := run.Kind
	if !kind.Valid() {
		kind = domain.RunKindPrimary
	}
	rootID := run.RootID
	if rootID == "" {
		rootID = run.ID
	}
	if _, err := tx.ExecContext(ctx,
		"INSERT INTO runs (id, session_id, status, created_at, kind, parent_run_id, root_run_id, depth) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)",
		run.ID, run.SessionID, string(run.Status), run.CreatedAt, string(kind), run.ParentID, rootID, run.Depth); err != nil {
		return domain.RunEvent{}, fmt.Errorf("storage: create primary run: %w", err)
	}

	started := admission.Started
	started.Seq = 1
	if _, err := tx.ExecContext(ctx,
		"INSERT INTO run_events (run_id, seq, type, created_at, payload_version, payload) VALUES ($1, $2, $3, $4, $5, $6)",
		started.RunID, int64(started.Seq), string(started.Type), started.CreatedAt, started.PayloadVersion, started.Payload); err != nil {
		return domain.RunEvent{}, fmt.Errorf("storage: append primary run.started: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return domain.RunEvent{}, fmt.Errorf("storage: commit primary run admission: %w", err)
	}
	return started, nil
}

// GetRun loads one run; absent ids yield storage.ErrNotFound.
func (b *Backend) GetRun(ctx context.Context, id domain.RunID) (domain.Run, error) {
	var r domain.Run
	var rid, sid, status, kind, parentID, rootID string
	err := b.db.QueryRowContext(ctx,
		`SELECT id, session_id, status, created_at, kind, parent_run_id, root_run_id, depth FROM runs WHERE id = ?`, id).
		Scan(&rid, &sid, &status, &r.CreatedAt, &kind, &parentID, &rootID, &r.Depth)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Run{}, storage.ErrNotFound
	}
	if err != nil {
		return domain.Run{}, fmt.Errorf("storage: get run %s: %w", id, err)
	}
	r.ID = domain.RunID(rid)
	r.SessionID = domain.SessionID(sid)
	r.Status = domain.RunStatus(status)
	r.Kind = domain.RunKind(kind)
	r.ParentID = domain.RunID(parentID)
	r.RootID = domain.RunID(rootID)
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
		`SELECT id, session_id, status, created_at, kind, parent_run_id, root_run_id, depth FROM runs
		 WHERE status IN `+activeStatuses+` ORDER BY created_at, id`)
	if err != nil {
		return nil, fmt.Errorf("storage: list active runs: %w", err)
	}
	defer func() { _ = rows.Close() }()

	out := []domain.Run{}
	for rows.Next() {
		var r domain.Run
		var rid, sid, status, kind, parentID, rootID string
		if err := rows.Scan(&rid, &sid, &status, &r.CreatedAt, &kind, &parentID, &rootID, &r.Depth); err != nil {
			return nil, fmt.Errorf("storage: scan run: %w", err)
		}
		r.ID = domain.RunID(rid)
		r.SessionID = domain.SessionID(sid)
		r.Status = domain.RunStatus(status)
		r.Kind = domain.RunKind(kind)
		r.ParentID = domain.RunID(parentID)
		r.RootID = domain.RunID(rootID)
		out = append(out, r)
	}
	return out, rows.Err()
}

// ListChildRuns returns direct children in creation order.
func (b *Backend) ListChildRuns(ctx context.Context, parentID domain.RunID) ([]domain.Run, error) {
	return b.listRunsWhere(ctx, `parent_run_id = ?`, parentID)
}

// ListRunTree returns all descendants of a root in creation order.
func (b *Backend) ListRunTree(ctx context.Context, rootID domain.RunID) ([]domain.Run, error) {
	return b.listRunsWhere(ctx, `root_run_id = ? AND id <> ?`, rootID, rootID)
}

// ListRunsBySession returns every run of a session in creation order,
// regardless of status (TT-1 session pin seeding).
func (b *Backend) ListRunsBySession(ctx context.Context, sessionID domain.SessionID) ([]domain.Run, error) {
	return b.listRunsWhere(ctx, `session_id = ?`, sessionID)
}

// LatestPrimaryRunBySession resolves one bounded workspace owner. Child runs
// never replace the primary conversation workspace in session/sidebar.
func (b *Backend) LatestPrimaryRunBySession(ctx context.Context, sessionID domain.SessionID) (domain.Run, error) {
	var r domain.Run
	var rid, sid, status, kind, parentID, rootID string
	err := b.db.QueryRowContext(ctx,
		`SELECT id, session_id, status, created_at, kind, parent_run_id, root_run_id, depth
		 FROM runs WHERE session_id = ? AND kind = ? ORDER BY created_at DESC, id DESC LIMIT 1`,
		sessionID, string(domain.RunKindPrimary)).
		Scan(&rid, &sid, &status, &r.CreatedAt, &kind, &parentID, &rootID, &r.Depth)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Run{}, storage.ErrNotFound
	}
	if err != nil {
		return domain.Run{}, fmt.Errorf("storage: latest primary run for session %s: %w", sessionID, err)
	}
	r.ID, r.SessionID, r.Status = domain.RunID(rid), domain.SessionID(sid), domain.RunStatus(status)
	r.Kind, r.ParentID, r.RootID = domain.RunKind(kind), domain.RunID(parentID), domain.RunID(rootID)
	return r, nil
}

func (b *Backend) listRunsWhere(ctx context.Context, predicate string, args ...any) ([]domain.Run, error) {
	rows, err := b.db.QueryContext(ctx,
		`SELECT id, session_id, status, created_at, kind, parent_run_id, root_run_id, depth
		 FROM runs WHERE `+predicate+` ORDER BY created_at, id`, args...)
	if err != nil {
		return nil, fmt.Errorf("storage: list run tree: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var out []domain.Run
	for rows.Next() {
		var r domain.Run
		var rid, sid, status, kind, parentID, rootID string
		if err := rows.Scan(&rid, &sid, &status, &r.CreatedAt, &kind, &parentID, &rootID, &r.Depth); err != nil {
			return nil, fmt.Errorf("storage: scan tree run: %w", err)
		}
		r.ID, r.SessionID, r.Status = domain.RunID(rid), domain.SessionID(sid), domain.RunStatus(status)
		r.Kind, r.ParentID, r.RootID = domain.RunKind(kind), domain.RunID(parentID), domain.RunID(rootID)
		out = append(out, r)
	}
	return out, rows.Err()
}
