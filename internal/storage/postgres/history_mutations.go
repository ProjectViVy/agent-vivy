package postgres

import (
	"context"
	"database/sql"
	"fmt"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/storage"
)

var _ storage.HistoryMutationStore = (*Backend)(nil)

func (b *Backend) CommitSessionRewind(ctx context.Context, marker storage.SessionTruncation, event domain.RunEvent) (domain.RunEvent, error) {
	tx, err := b.db.SQL.BeginTx(ctx, nil)
	if err != nil {
		return event, fmt.Errorf("storage: begin session rewind: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	if err := postgresInsertMarker(ctx, tx, marker); err != nil {
		return event, err
	}
	if err := postgresInsertHistoryEvent(ctx, tx, &event); err != nil {
		return event, err
	}
	if err := tx.Commit(); err != nil {
		return event, fmt.Errorf("storage: commit session rewind: %w", err)
	}
	return event, nil
}

func (b *Backend) CommitSessionFork(ctx context.Context, child domain.Session, messages []domain.Message, markers []storage.SessionTruncation, events []domain.RunEvent) ([]domain.RunEvent, error) {
	tx, err := b.db.SQL.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("storage: begin session fork: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	mode, policy := child.EffectiveSandbox()
	if _, err := tx.ExecContext(ctx, `INSERT INTO sessions (id,title,created_at,sandbox_mode,approval_policy) VALUES ($1,$2,$3,$4,$5)`, child.ID, child.Title, child.CreatedAt, mode, policy); err != nil {
		return nil, fmt.Errorf("storage: create fork session: %w", err)
	}
	for _, m := range messages {
		if _, err := tx.ExecContext(ctx, `INSERT INTO messages (id,session_id,run_id,role,created_at,content,tool_call_id,tool_name,tool_args,source,channel,chat_id,channel_message_id) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)`, m.ID, m.SessionID, m.RunID, m.Role, m.CreatedAt, m.Content, m.ToolCallID, m.ToolName, toolArgsBlob(m.ToolArgs), m.Source, m.Channel, m.ChatID, m.ChannelMessageID); err != nil {
			return nil, fmt.Errorf("storage: copy fork message: %w", err)
		}
		for i, a := range m.Attachments {
			if _, err := tx.ExecContext(ctx, `INSERT INTO message_attachments (message_id,position,name,mime_type,data) VALUES ($1,$2,$3,$4,$5)`, m.ID, i, a.Name, a.MimeType, a.Data); err != nil {
				return nil, fmt.Errorf("storage: copy fork attachment: %w", err)
			}
		}
	}
	for _, marker := range markers {
		if err := postgresInsertMarker(ctx, tx, marker); err != nil {
			return nil, err
		}
	}
	for i := range events {
		if err := postgresInsertHistoryEvent(ctx, tx, &events[i]); err != nil {
			return nil, err
		}
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("storage: commit session fork: %w", err)
	}
	return events, nil
}

func postgresInsertMarker(ctx context.Context, tx *sql.Tx, t storage.SessionTruncation) error {
	if _, err := tx.ExecContext(ctx, `INSERT INTO session_truncations (session_id,cutoff_message_id,tail_message_id,reason,fork_session_id,created_at) VALUES ($1,$2,$3,$4,$5,$6)`, t.SessionID, t.CutoffMessageID, t.TailMessageID, t.Reason, t.ForkSessionID, t.CreatedAt); err != nil {
		return fmt.Errorf("storage: record session truncation: %w", err)
	}
	return nil
}

func postgresInsertHistoryEvent(ctx context.Context, tx *sql.Tx, e *domain.RunEvent) error {
	e.Seq = 1
	if _, err := tx.ExecContext(ctx, `INSERT INTO run_events (run_id,seq,type,created_at,payload_version,payload) VALUES ($1,$2,$3,$4,$5,$6)`, e.RunID, e.Seq, e.Type, e.CreatedAt, e.PayloadVersion, e.Payload); err != nil {
		return fmt.Errorf("storage: insert history event: %w", err)
	}
	return nil
}
