package postgres

import (
	"context"
	"fmt"

	"agent-vivy/internal/domain"
)

// AppendMessage inserts one append-only conversation turn (FR-2: there
// is no update path for message content). Image attachments (VC-1g-2)
// are persisted in the same transaction as their message row so a
// partial write can never orphan bytes.
func (b *Backend) AppendMessage(ctx context.Context, m domain.Message) error {
	tx, err := b.db.SQL.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("storage: begin append message %s: %w", m.ID, err)
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO messages (id, session_id, run_id, role, created_at, content, tool_call_id, tool_name, tool_args, source, channel, chat_id, channel_message_id)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)`,
		m.ID, m.SessionID, m.RunID, string(m.Role), m.CreatedAt, m.Content,
		m.ToolCallID, m.ToolName, toolArgsBlob(m.ToolArgs),
		m.Source, m.Channel, m.ChatID, m.ChannelMessageID); err != nil {
		return fmt.Errorf("storage: append message %s: %w", m.ID, err)
	}
	for position, attachment := range m.Attachments {
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO message_attachments (message_id, position, name, mime_type, data)
			 VALUES ($1, $2, $3, $4, $5)`,
			m.ID, position, attachment.Name, attachment.MimeType, attachment.Data); err != nil {
			return fmt.Errorf("storage: append message %s attachment %d: %w", m.ID, position, err)
		}
	}
	for position, file := range m.FileContexts {
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO message_file_contexts (message_id, position, path, name, size, content)
			 VALUES ($1, $2, $3, $4, $5, $6)`,
			m.ID, position, file.Path, file.Name, file.Size, file.Content); err != nil {
			return fmt.Errorf("storage: append message %s file context %d: %w", m.ID, position, err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("storage: commit append message %s: %w", m.ID, err)
	}
	return nil
}

// ListMessages returns the session's messages in creation order. An
// unknown session yields an empty list (existence is the caller's
// concern). Image attachments load with their message rows.
func (b *Backend) ListMessages(ctx context.Context, sessionID domain.SessionID) ([]domain.Message, error) {
	rows, err := b.db.SQL.QueryContext(ctx,
		`SELECT id, session_id, run_id, role, created_at, content, tool_call_id, tool_name, tool_args, source, channel, chat_id, channel_message_id
		 FROM messages WHERE session_id = $1 ORDER BY created_at, id`, sessionID)
	if err != nil {
		return nil, fmt.Errorf("storage: list messages %s: %w", sessionID, err)
	}
	defer func() { _ = rows.Close() }()

	out := []domain.Message{}
	byID := make(map[string]int)
	for rows.Next() {
		var m domain.Message
		var id, sid, rid, role string
		var args []byte
		if err := rows.Scan(&id, &sid, &rid, &role, &m.CreatedAt, &m.Content, &m.ToolCallID, &m.ToolName, &args,
			&m.Source, &m.Channel, &m.ChatID, &m.ChannelMessageID); err != nil {
			return nil, fmt.Errorf("storage: scan message: %w", err)
		}
		m.ID = id
		m.SessionID = domain.SessionID(sid)
		m.RunID = domain.RunID(rid)
		m.Role = domain.Role(role)
		if len(args) > 0 {
			m.ToolArgs = args
		}
		byID[id] = len(out)
		out = append(out, m)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("storage: list messages %s: %w", sessionID, err)
	}
	if err := b.listAttachments(ctx, sessionID, out, byID); err != nil {
		return nil, err
	}
	if err := b.listFileContexts(ctx, sessionID, out, byID); err != nil {
		return nil, err
	}
	return out, nil
}

// listAttachments loads the session's image attachments and fills them
// into the already-scanned message rows by message id.
func (b *Backend) listAttachments(ctx context.Context, sessionID domain.SessionID, out []domain.Message, byID map[string]int) error {
	rows, err := b.db.SQL.QueryContext(ctx,
		`SELECT a.message_id, a.name, a.mime_type, a.data
		 FROM message_attachments a JOIN messages m ON m.id = a.message_id
		 WHERE m.session_id = $1 ORDER BY a.message_id, a.position`, sessionID)
	if err != nil {
		return fmt.Errorf("storage: list message attachments %s: %w", sessionID, err)
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var messageID, name, mimeType string
		var data []byte
		if err := rows.Scan(&messageID, &name, &mimeType, &data); err != nil {
			return fmt.Errorf("storage: scan message attachment: %w", err)
		}
		index, ok := byID[messageID]
		if !ok {
			continue
		}
		out[index].Attachments = append(out[index].Attachments, domain.Attachment{Name: name, MimeType: mimeType, Data: data})
	}
	return rows.Err()
}

// listFileContexts loads the durable project-file snapshots for the session.
// Content is used only by runtime context construction; RPC projections strip
// it before returning history to a face.
func (b *Backend) listFileContexts(ctx context.Context, sessionID domain.SessionID, out []domain.Message, byID map[string]int) error {
	rows, err := b.db.SQL.QueryContext(ctx,
		`SELECT c.message_id, c.path, c.name, c.size, c.content
		 FROM message_file_contexts c JOIN messages m ON m.id = c.message_id
		 WHERE m.session_id = $1 ORDER BY c.message_id, c.position`, sessionID)
	if err != nil {
		return fmt.Errorf("storage: list message file contexts %s: %w", sessionID, err)
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var messageID, path, name string
		var size int64
		var content []byte
		if err := rows.Scan(&messageID, &path, &name, &size, &content); err != nil {
			return fmt.Errorf("storage: scan message file context: %w", err)
		}
		index, ok := byID[messageID]
		if !ok {
			continue
		}
		body := make([]byte, len(content))
		copy(body, content)
		out[index].FileContexts = append(out[index].FileContexts, domain.FileContext{
			Path: path, Name: name, Size: size, Content: body,
		})
	}
	return rows.Err()
}

func toolArgsBlob(args []byte) []byte {
	if args == nil {
		return []byte{}
	}
	return args
}
