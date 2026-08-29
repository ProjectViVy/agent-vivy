package sqlite

import (
	"context"
	"fmt"

	"agent-vivy/internal/domain"
)

// AppendMessage inserts one append-only conversation turn (FR-2: there
// is no update path for message content).
func (b *Backend) AppendMessage(ctx context.Context, m domain.Message) error {
	if _, err := b.db.ExecContext(ctx,
		`INSERT INTO messages (id, session_id, run_id, role, created_at, content, tool_call_id, tool_name, tool_args, source, channel, chat_id, channel_message_id)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		m.ID, m.SessionID, m.RunID, string(m.Role), m.CreatedAt, m.Content,
		m.ToolCallID, m.ToolName, toolArgsBlob(m.ToolArgs),
		m.Source, m.Channel, m.ChatID, m.ChannelMessageID); err != nil {
		return fmt.Errorf("storage: append message %s: %w", m.ID, err)
	}
	return nil
}

// ListMessages returns the session's messages in creation order. An
// unknown session yields an empty list (existence is the caller's
// concern).
func (b *Backend) ListMessages(ctx context.Context, sessionID domain.SessionID) ([]domain.Message, error) {
	rows, err := b.db.QueryContext(ctx,
		`SELECT id, session_id, run_id, role, created_at, content, tool_call_id, tool_name, tool_args, source, channel, chat_id, channel_message_id
		 FROM messages WHERE session_id = ? ORDER BY created_at, id`, sessionID)
	if err != nil {
		return nil, fmt.Errorf("storage: list messages %s: %w", sessionID, err)
	}
	defer func() { _ = rows.Close() }()

	out := []domain.Message{}
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
		out = append(out, m)
	}
	return out, rows.Err()
}

func toolArgsBlob(args []byte) []byte {
	if args == nil {
		return []byte{}
	}
	return args
}
