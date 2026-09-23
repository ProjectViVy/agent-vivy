package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"agent-vivy/internal/domain"
	mask "agent-vivy/internal/maskcontract"
	"agent-vivy/internal/storage"
)

var _ storage.HistoryMutationStore = (*Backend)(nil)

func (b *Backend) CommitSessionRewind(ctx context.Context, marker storage.SessionTruncation, event domain.RunEvent) (domain.RunEvent, error) {
	tx, err := b.db.BeginTx(ctx, nil)
	if err != nil {
		return event, fmt.Errorf("storage: begin session rewind: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	if err := sqliteInsertMarker(ctx, tx, marker); err != nil {
		return event, err
	}
	if err := sqliteInsertHistoryEvent(ctx, tx, &event); err != nil {
		return event, err
	}
	at := messageActivityAt(event.CreatedAt)
	if _, err := tx.ExecContext(ctx, `UPDATE sessions SET updated_at = CASE WHEN updated_at < ? THEN ? ELSE updated_at END WHERE id = ?`, at, at, marker.SessionID); err != nil {
		return event, fmt.Errorf("storage: touch rewound session: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return event, fmt.Errorf("storage: commit session rewind: %w", err)
	}
	return event, nil
}

func (b *Backend) CommitSessionEdit(ctx context.Context, marker storage.SessionTruncation, m domain.Message, run domain.Run, event domain.RunEvent) (domain.RunEvent, error) {
	tx, err := b.db.BeginTx(ctx, nil)
	if err != nil {
		return event, err
	}
	defer func() { _ = tx.Rollback() }()
	if err := sqliteInsertMarker(ctx, tx, marker); err != nil {
		return event, err
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO messages (id,session_id,run_id,role,created_at,content,tool_call_id,tool_name,tool_args,source,channel,chat_id,channel_message_id) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?)`, m.ID, m.SessionID, m.RunID, m.Role, m.CreatedAt, m.Content, m.ToolCallID, m.ToolName, toolArgsBlob(m.ToolArgs), m.Source, m.Channel, m.ChatID, m.ChannelMessageID); err != nil {
		return event, err
	}
	for i, a := range m.Attachments {
		if _, err := tx.ExecContext(ctx, `INSERT INTO message_attachments (message_id,position,name,mime_type,data) VALUES (?,?,?,?,?)`, m.ID, i, a.Name, a.MimeType, a.Data); err != nil {
			return event, err
		}
	}
	for i, file := range m.FileContexts {
		if _, err := tx.ExecContext(ctx, `INSERT INTO message_file_contexts (message_id,position,path,name,size,content) VALUES (?,?,?,?,?,?)`, m.ID, i, file.Path, file.Name, file.Size, file.Content); err != nil {
			return event, err
		}
	}
	at := messageActivityAt(m.CreatedAt)
	if _, err := tx.ExecContext(ctx, `UPDATE sessions SET updated_at = CASE WHEN updated_at < ? THEN ? ELSE updated_at END WHERE id = ?`, at, at, m.SessionID); err != nil {
		return event, fmt.Errorf("storage: touch edited session: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO runs (id,session_id,status,created_at,kind,parent_run_id,root_run_id,depth) VALUES (?,?,?,?,?,?,?,?)`, run.ID, run.SessionID, domain.RunActive, run.CreatedAt, domain.RunKindPrimary, "", run.ID, 0); err != nil {
		return event, err
	}
	if err := sqliteInsertHistoryEvent(ctx, tx, &event); err != nil {
		return event, err
	}
	if err := tx.Commit(); err != nil {
		return event, err
	}
	return event, nil
}

func (b *Backend) CommitSessionFork(ctx context.Context, child domain.Session, messages []domain.Message, markers []storage.SessionTruncation, events []domain.RunEvent) ([]domain.RunEvent, error) {
	tx, err := b.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("storage: begin session fork: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	mode, policy := child.EffectiveSandbox()
	updatedAt := child.UpdatedAt
	if updatedAt <= 0 {
		updatedAt = child.CreatedAt
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO sessions (id,title,created_at,updated_at,sandbox_mode,approval_policy,workspace_path) VALUES (?,?,?,?,?,?,?)`, child.ID, child.Title, child.CreatedAt, updatedAt, mode, policy, child.WorkspacePath); err != nil {
		return nil, fmt.Errorf("storage: create fork session: %w", err)
	}
	if err := sqliteCopyForkMaskSelection(ctx, tx, child.ID, markers); err != nil {
		return nil, err
	}
	for _, m := range messages {
		if _, err := tx.ExecContext(ctx, `INSERT INTO messages (id,session_id,run_id,role,created_at,content,tool_call_id,tool_name,tool_args,source,channel,chat_id,channel_message_id) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?)`, m.ID, m.SessionID, m.RunID, m.Role, m.CreatedAt, m.Content, m.ToolCallID, m.ToolName, toolArgsBlob(m.ToolArgs), m.Source, m.Channel, m.ChatID, m.ChannelMessageID); err != nil {
			return nil, fmt.Errorf("storage: copy fork message: %w", err)
		}
		for i, a := range m.Attachments {
			if _, err := tx.ExecContext(ctx, `INSERT INTO message_attachments (message_id,position,name,mime_type,data) VALUES (?,?,?,?,?)`, m.ID, i, a.Name, a.MimeType, a.Data); err != nil {
				return nil, fmt.Errorf("storage: copy fork attachment: %w", err)
			}
		}
		for i, file := range m.FileContexts {
			if _, err := tx.ExecContext(ctx, `INSERT INTO message_file_contexts (message_id,position,path,name,size,content) VALUES (?,?,?,?,?,?)`, m.ID, i, file.Path, file.Name, file.Size, file.Content); err != nil {
				return nil, fmt.Errorf("storage: copy fork file context: %w", err)
			}
		}
	}
	for _, marker := range markers {
		if err := sqliteInsertMarker(ctx, tx, marker); err != nil {
			return nil, err
		}
	}
	for i := range events {
		if err := sqliteInsertHistoryEvent(ctx, tx, &events[i]); err != nil {
			return nil, err
		}
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("storage: commit session fork: %w", err)
	}
	return events, nil
}

// sqliteCopyForkMaskSelection copies the source session's current selection
// into a newly-created child as an independent revision-1 row. The source is
// identified by the provenance marker assembled by runtime/rewind_service.
// A missing source selection is the virtual unmasked revision 0, which still
// becomes an explicit empty revision 1 on the child.
func sqliteCopyForkMaskSelection(ctx context.Context, tx *sql.Tx, childID domain.SessionID, markers []storage.SessionTruncation) error {
	sourceID := sqliteForkSourceID(childID, markers)
	if sourceID == "" {
		return nil
	}
	var lockedID string
	if _, err := tx.ExecContext(ctx, `UPDATE sessions SET id = id WHERE id = ?`, sourceID); err != nil {
		return fmt.Errorf("storage: lock fork source session: %w", err)
	}
	if err := tx.QueryRowContext(ctx, `SELECT id FROM sessions WHERE id = ?`, sourceID).Scan(&lockedID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return storage.ErrNotFound
		}
		return fmt.Errorf("storage: read fork source session: %w", err)
	}
	selection, err := sqliteReadSelection(ctx, tx, sourceID)
	if err != nil {
		return err
	}
	if selection.MaskID != "" && !mask.IsBuiltinID(selection.MaskID) {
		if _, err := sqliteGetCustomMask(ctx, tx, selection.MaskID); err != nil {
			return fmt.Errorf("storage: lock fork mask definition: %w", err)
		}
	}
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO session_mask_selections (session_id, mask_id, revision) VALUES (?, ?, 1)`,
		childID, selection.MaskID); err != nil {
		return fmt.Errorf("storage: copy fork mask selection: %w", err)
	}
	return nil
}

func sqliteForkSourceID(childID domain.SessionID, markers []storage.SessionTruncation) domain.SessionID {
	for _, marker := range markers {
		if marker.Reason == storage.TruncationFork && marker.ForkSessionID == string(childID) {
			return marker.SessionID
		}
	}
	for _, marker := range markers {
		if marker.Reason == storage.TruncationForkedFrom && marker.SessionID == childID {
			return domain.SessionID(marker.ForkSessionID)
		}
	}
	return ""
}

func sqliteInsertMarker(ctx context.Context, tx *sql.Tx, t storage.SessionTruncation) error {
	if _, err := tx.ExecContext(ctx, `INSERT INTO session_truncations (session_id,run_id,cutoff_message_id,tail_message_id,reason,fork_session_id,created_at) VALUES (?,?,?,?,?,?,?)`, t.SessionID, t.RunID, t.CutoffMessageID, t.TailMessageID, t.Reason, t.ForkSessionID, t.CreatedAt); err != nil {
		return fmt.Errorf("storage: record session truncation: %w", err)
	}
	return nil
}

func sqliteInsertHistoryEvent(ctx context.Context, tx *sql.Tx, e *domain.RunEvent) error {
	e.Seq = 1
	if _, err := tx.ExecContext(ctx, `INSERT INTO run_events (run_id,seq,type,created_at,payload_version,payload) VALUES (?,?,?,?,?,?)`, e.RunID, e.Seq, e.Type, e.CreatedAt, e.PayloadVersion, e.Payload); err != nil {
		return fmt.Errorf("storage: insert history event: %w", err)
	}
	return nil
}
