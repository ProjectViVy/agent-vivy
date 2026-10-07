package postgres

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"fmt"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/storage"
)

// CommitChannelTask mirrors the SQLite implementation: the scope row lock
// (SELECT ... FOR UPDATE) serializes concurrent retries, the receipt row
// decides replay-vs-conflict before any admission write, and the whole
// envelope — session, context, message, run, prompt, events, receipt —
// commits or rolls back together.
func (b *Backend) CommitChannelTask(ctx context.Context, commit storage.ChannelTaskCommit) (storage.ChannelTaskCommitResult, error) {
	if err := storage.ValidateChannelTaskCommit(commit); err != nil {
		return storage.ChannelTaskCommitResult{}, err
	}
	tx, err := b.db.BeginTx(ctx, nil)
	if err != nil {
		return storage.ChannelTaskCommitResult{}, fmt.Errorf("storage: begin channel task admission: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	scope := commit.Scope
	createdAt := commit.Message.CreatedAt
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO channel_task_scopes (instance_key, principal_id, created_at) VALUES (?, ?, ?) ON CONFLICT (instance_key, principal_id) DO NOTHING`,
		scope.InstanceKey, scope.PrincipalID, createdAt); err != nil {
		return storage.ChannelTaskCommitResult{}, fmt.Errorf("storage: ensure channel task scope: %w", err)
	}
	var lockedInstance string
	if err := tx.QueryRowContext(ctx,
		`SELECT instance_key FROM channel_task_scopes WHERE instance_key = ? AND principal_id = ? FOR UPDATE`,
		scope.InstanceKey, scope.PrincipalID).Scan(&lockedInstance); errors.Is(err, sql.ErrNoRows) {
		return storage.ChannelTaskCommitResult{}, storage.ErrNotFound
	} else if err != nil {
		return storage.ChannelTaskCommitResult{}, fmt.Errorf("storage: lock channel task scope: %w", err)
	}

	var storedHash []byte
	var receipt domain.ChannelTaskReceipt
	var questionID sql.NullString
	var deletedAt sql.NullInt64
	err = tx.QueryRowContext(ctx,
		`SELECT input_hash, operation, session_id, run_id, question_id, accepted_seq, created_at, deleted_at
		 FROM channel_task_receipts
		 WHERE instance_key = ? AND principal_id = ? AND message_id = ?`,
		scope.InstanceKey, scope.PrincipalID, commit.MessageID).
		Scan(&storedHash, &receipt.Operation, &receipt.SessionID, &receipt.RunID, &questionID,
			&receipt.AcceptedSeq, &receipt.CreatedAt, &deletedAt)
	reviveReceipt := false
	switch {
	case err == nil:
		if !deletedAt.Valid {
			if !bytes.Equal(storedHash, commit.InputHash[:]) {
				return storage.ChannelTaskCommitResult{}, storage.ErrConflict
			}
			if err := tx.Commit(); err != nil {
				return storage.ChannelTaskCommitResult{}, fmt.Errorf("storage: replay channel task admission: %w", err)
			}
			receipt.Scope = scope
			receipt.MessageID = commit.MessageID
			receipt.InputHash = commit.InputHash
			receipt.QuestionID = questionID.String
			return storage.ChannelTaskCommitResult{Receipt: receipt, NewlyCommitted: false}, nil
		}
		// The receipt is tombstoned: the same dedup key may admit again,
		// reviving the row with the new execution identity. The deleted
		// session stays dead either way.
		reviveReceipt = true
	case !errors.Is(err, sql.ErrNoRows):
		return storage.ChannelTaskCommitResult{}, fmt.Errorf("storage: read channel task receipt: %w", err)
	}

	if commit.NewSession != nil {
		s := *commit.NewSession
		mode, policy := s.EffectiveSandbox()
		if s.UpdatedAt <= 0 {
			s.UpdatedAt = s.CreatedAt
		}
		var exists int
		if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM sessions WHERE id = ?`, s.ID).Scan(&exists); err != nil {
			return storage.ChannelTaskCommitResult{}, fmt.Errorf("storage: check channel task session: %w", err)
		}
		if exists != 0 {
			return storage.ChannelTaskCommitResult{}, storage.ErrConflict
		}
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO sessions (id, title, created_at, updated_at, sandbox_mode, approval_policy, workspace_path) VALUES (?, ?, ?, ?, ?, ?, ?)`,
			s.ID, s.Title, s.CreatedAt, s.UpdatedAt, string(mode), string(policy), s.WorkspacePath); err != nil {
			return storage.ChannelTaskCommitResult{}, fmt.Errorf("storage: create channel task session: %w", err)
		}
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO channel_task_contexts (session_id, instance_key, principal_id, created_at) VALUES (?, ?, ?, ?)`,
			s.ID, scope.InstanceKey, scope.PrincipalID, createdAt); err != nil {
			return storage.ChannelTaskCommitResult{}, fmt.Errorf("storage: create channel task context: %w", err)
		}
	} else {
		var ownerInstance, ownerPrincipal string
		err := tx.QueryRowContext(ctx,
			`SELECT instance_key, principal_id FROM channel_task_contexts WHERE session_id = ? AND deleted_at IS NULL`,
			commit.Run.SessionID).Scan(&ownerInstance, &ownerPrincipal)
		if errors.Is(err, sql.ErrNoRows) {
			return storage.ChannelTaskCommitResult{}, storage.ErrNotFound
		} else if err != nil {
			return storage.ChannelTaskCommitResult{}, fmt.Errorf("storage: read channel task context: %w", err)
		}
		if ownerInstance != scope.InstanceKey || ownerPrincipal != scope.PrincipalID {
			return storage.ChannelTaskCommitResult{}, storage.ErrNotFound
		}
		var sessionID string
		if err := tx.QueryRowContext(ctx,
			`SELECT id FROM sessions WHERE id = ? FOR UPDATE`, commit.Run.SessionID).Scan(&sessionID); errors.Is(err, sql.ErrNoRows) {
			return storage.ChannelTaskCommitResult{}, storage.ErrNotFound
		} else if err != nil {
			return storage.ChannelTaskCommitResult{}, fmt.Errorf("storage: lock channel task session: %w", err)
		}
	}

	admission := commit.PrimaryRunCommit
	if err := b.postgresValidateAdmissionCapture(ctx, tx, admission.ExpectedMask); err != nil {
		return storage.ChannelTaskCommitResult{}, err
	}
	message := admission.Message
	position, err := postgresNextMessagePosition(ctx, tx.SQL, message.SessionID)
	if err != nil {
		return storage.ChannelTaskCommitResult{}, err
	}
	message.WorkSeq, err = currentMessageWorkSeq(ctx, tx.SQL, message.SessionID)
	if err != nil {
		return storage.ChannelTaskCommitResult{}, err
	}

	var active int
	if err := tx.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM runs WHERE session_id = ? AND kind = ? AND status IN "+activeStatuses,
		admission.Run.SessionID, string(domain.RunKindPrimary)).Scan(&active); err != nil {
		return storage.ChannelTaskCommitResult{}, fmt.Errorf("storage: inspect active channel task run: %w", err)
	}
	if active != 0 {
		return storage.ChannelTaskCommitResult{}, storage.ErrWorkRunConflict
	}

	if _, err := tx.ExecContext(ctx,
		`INSERT INTO messages (id, session_id, run_id, role, created_at, work_seq, content, tool_call_id, tool_name, tool_args, source, channel, chat_id, channel_message_id, position) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		message.ID, message.SessionID, message.RunID, string(message.Role), message.CreatedAt, int64(message.WorkSeq), []byte(message.Content),
		message.ToolCallID, message.ToolName, toolArgsBlob(message.ToolArgs),
		message.Source, message.Channel, message.ChatID, message.ChannelMessageID, position); err != nil {
		return storage.ChannelTaskCommitResult{}, fmt.Errorf("storage: append channel task message: %w", err)
	}
	for position, attachment := range message.Attachments {
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO message_attachments (message_id, position, name, mime_type, data) VALUES (?, ?, ?, ?, ?)`,
			message.ID, position, attachment.Name, attachment.MimeType, attachment.Data); err != nil {
			return storage.ChannelTaskCommitResult{}, fmt.Errorf("storage: append channel task attachment: %w", err)
		}
	}
	for position, file := range message.FileContexts {
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO message_file_contexts (message_id, position, path, name, size, content) VALUES (?, ?, ?, ?, ?, ?)`,
			message.ID, position, file.Path, file.Name, file.Size, file.Content); err != nil {
			return storage.ChannelTaskCommitResult{}, fmt.Errorf("storage: append channel task file context: %w", err)
		}
	}
	at := messageActivityAt(message.CreatedAt)
	if _, err := tx.ExecContext(ctx,
		`UPDATE sessions SET updated_at = CASE WHEN updated_at < ? THEN ? ELSE updated_at END WHERE id = ?`,
		at, at, message.SessionID); err != nil {
		return storage.ChannelTaskCommitResult{}, fmt.Errorf("storage: touch channel task session: %w", err)
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
		`INSERT INTO runs (id, session_id, status, created_at, kind, parent_run_id, root_run_id, depth) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		run.ID, run.SessionID, string(run.Status), run.CreatedAt, string(kind), run.ParentID, rootID, run.Depth); err != nil {
		return storage.ChannelTaskCommitResult{}, fmt.Errorf("storage: create channel task run: %w", err)
	}
	if admission.Prompt != nil {
		if err := postgresInsertAdmissionPrompt(ctx, tx, *admission.Prompt); err != nil {
			return storage.ChannelTaskCommitResult{}, err
		}
	}

	started := admission.Started
	started.Seq = 1
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO run_events (run_id, seq, type, created_at, payload_version, payload) VALUES (?, ?, ?, ?, ?, ?)`,
		started.RunID, int64(started.Seq), string(started.Type), started.CreatedAt, started.PayloadVersion, started.Payload); err != nil {
		return storage.ChannelTaskCommitResult{}, fmt.Errorf("storage: journal channel task run.started: %w", err)
	}
	admitted := commit.Admitted
	admitted.Seq = 2
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO run_events (run_id, seq, type, created_at, payload_version, payload) VALUES (?, ?, ?, ?, ?, ?)`,
		admitted.RunID, int64(admitted.Seq), string(admitted.Type), admitted.CreatedAt, admitted.PayloadVersion, admitted.Payload); err != nil {
		return storage.ChannelTaskCommitResult{}, fmt.Errorf("storage: journal channel task admitted: %w", err)
	}

	if reviveReceipt {
		if _, err := tx.ExecContext(ctx,
			`UPDATE channel_task_receipts SET input_hash = ?, session_id = ?, run_id = ?, question_id = NULL, accepted_seq = ?, created_at = ?, deleted_at = NULL
			 WHERE instance_key = ? AND principal_id = ? AND message_id = ?`,
			commit.InputHash[:], message.SessionID, run.ID, admitted.Seq, createdAt,
			scope.InstanceKey, scope.PrincipalID, commit.MessageID); err != nil {
			return storage.ChannelTaskCommitResult{}, fmt.Errorf("storage: revive channel task receipt: %w", err)
		}
	} else if _, err := tx.ExecContext(ctx,
		`INSERT INTO channel_task_receipts (instance_key, principal_id, message_id, input_hash, operation, session_id, run_id, question_id, accepted_seq, created_at)
		 VALUES (?, ?, ?, ?, 'submit', ?, ?, NULL, ?, ?)`,
		scope.InstanceKey, scope.PrincipalID, commit.MessageID, commit.InputHash[:],
		message.SessionID, run.ID, admitted.Seq, createdAt); err != nil {
		return storage.ChannelTaskCommitResult{}, fmt.Errorf("storage: write channel task receipt: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return storage.ChannelTaskCommitResult{}, fmt.Errorf("storage: commit channel task admission: %w", err)
	}
	receipt = domain.ChannelTaskReceipt{
		Scope: scope, MessageID: commit.MessageID, Operation: "submit",
		InputHash: commit.InputHash, SessionID: message.SessionID, RunID: run.ID,
		AcceptedSeq: admitted.Seq, CreatedAt: createdAt,
	}
	return storage.ChannelTaskCommitResult{
		Receipt:        receipt,
		Events:         []domain.RunEvent{started, admitted},
		NewlyCommitted: true,
	}, nil
}

// FindChannelTaskReceipt — see the SQLite twin; tombstoned rows are not results.
func (b *Backend) FindChannelTaskReceipt(ctx context.Context, scope domain.ChannelTaskScope, messageID string) (domain.ChannelTaskReceipt, bool, error) {
	var (
		receipt    domain.ChannelTaskReceipt
		storedHash []byte
		questionID sql.NullString
		deletedAt  sql.NullInt64
	)
	err := b.db.QueryRowContext(ctx,
		`SELECT input_hash, operation, session_id, run_id, question_id, accepted_seq, created_at, deleted_at
		 FROM channel_task_receipts
		 WHERE instance_key = ? AND principal_id = ? AND message_id = ?`,
		scope.InstanceKey, scope.PrincipalID, messageID).
		Scan(&storedHash, &receipt.Operation, &receipt.SessionID, &receipt.RunID, &questionID,
			&receipt.AcceptedSeq, &receipt.CreatedAt, &deletedAt)
	if errors.Is(err, sql.ErrNoRows) || deletedAt.Valid {
		return domain.ChannelTaskReceipt{}, false, nil
	} else if err != nil {
		return domain.ChannelTaskReceipt{}, false, fmt.Errorf("storage: find channel task receipt: %w", err)
	}
	receipt.Scope = scope
	receipt.MessageID = messageID
	receipt.QuestionID = questionID.String
	copy(receipt.InputHash[:], storedHash)
	return receipt, true, nil
}

// GetChannelTaskOwner — foreign or missing addresses are ErrNotFound.
func (b *Backend) GetChannelTaskOwner(ctx context.Context, scope domain.ChannelTaskScope, runID domain.RunID) (domain.SessionID, error) {
	var sessionID domain.SessionID
	err := b.db.QueryRowContext(ctx,
		`SELECT session_id FROM channel_task_receipts
		 WHERE instance_key = ? AND principal_id = ? AND operation = 'submit' AND run_id = ? AND deleted_at IS NULL`,
		scope.InstanceKey, scope.PrincipalID, runID).Scan(&sessionID)
	if errors.Is(err, sql.ErrNoRows) {
		return "", storage.ErrNotFound
	} else if err != nil {
		return "", fmt.Errorf("storage: channel task owner %s: %w", runID, err)
	}
	return sessionID, nil
}

// ListChannelTaskRuns — see the SQLite twin; identical query and cursor.
func (b *Backend) ListChannelTaskRuns(ctx context.Context, query storage.ChannelTaskRunQuery) (storage.ChannelTaskRunPage, error) {
	limit := query.Limit
	if limit <= 0 {
		limit = 50
	}
	args := []any{query.Scope.InstanceKey, query.Scope.PrincipalID}
	where := `WHERE r.instance_key = ? AND r.principal_id = ? AND r.operation = 'submit' AND r.deleted_at IS NULL`
	if query.SessionID != "" {
		where += ` AND r.session_id = ?`
		args = append(args, query.SessionID)
	}
	if query.AfterRunID != "" {
		where += ` AND (r.created_at, r.run_id) < (SELECT created_at, run_id FROM channel_task_receipts WHERE instance_key = ? AND principal_id = ? AND operation = 'submit' AND run_id = ?)`
		args = append(args, query.Scope.InstanceKey, query.Scope.PrincipalID, query.AfterRunID)
	}
	rows, err := b.db.QueryContext(ctx,
		`SELECT r.run_id, r.session_id, r.created_at FROM channel_task_receipts r
		 `+where+` ORDER BY r.created_at DESC, r.run_id DESC LIMIT ?`,
		append(args, limit+1)...)
	if err != nil {
		return storage.ChannelTaskRunPage{}, fmt.Errorf("storage: list channel task runs: %w", err)
	}
	defer func() { _ = rows.Close() }()

	type row struct {
		runID     domain.RunID
		sessionID domain.SessionID
	}
	var scanned []row
	for rows.Next() {
		var r row
		var created int64
		if err := rows.Scan(&r.runID, &r.sessionID, &created); err != nil {
			return storage.ChannelTaskRunPage{}, fmt.Errorf("storage: scan channel task run: %w", err)
		}
		scanned = append(scanned, r)
	}
	if err := rows.Err(); err != nil {
		return storage.ChannelTaskRunPage{}, err
	}
	page := storage.ChannelTaskRunPage{}
	if len(scanned) > limit {
		page.HasMore = true
		scanned = scanned[:limit]
	}
	for _, r := range scanned {
		run, err := b.GetRun(ctx, r.runID)
		if err != nil {
			if errors.Is(err, storage.ErrNotFound) {
				// A non-tombstoned receipt pointing at a missing run is
				// corruption: deletion tombstones before removing rows.
				return storage.ChannelTaskRunPage{}, fmt.Errorf("storage: channel task receipt for run %s has no committed run", r.runID)
			}
			return storage.ChannelTaskRunPage{}, err
		}
		page.Runs = append(page.Runs, run)
		page.NextRunID = r.runID
	}
	return page, nil
}
