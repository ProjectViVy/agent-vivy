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
		`INSERT INTO messages (id, session_id, run_id, role, created_at, content)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		m.ID, m.SessionID, m.RunID, string(m.Role), m.CreatedAt, m.Content); err != nil {
		return fmt.Errorf("storage: append message %s: %w", m.ID, err)
	}
	return nil
}

// ListMessages returns the session's messages in creation order. An
// unknown session yields an empty list (existence is the caller's
// concern).
func (b *Backend) ListMessages(ctx context.Context, sessionID domain.SessionID) ([]domain.Message, error) {
	rows, err := b.db.QueryContext(ctx,
		`SELECT id, session_id, run_id, role, created_at, content
		 FROM messages WHERE session_id = ? ORDER BY created_at, id`, sessionID)
	if err != nil {
		return nil, fmt.Errorf("storage: list messages %s: %w", sessionID, err)
	}
	defer func() { _ = rows.Close() }()

	out := []domain.Message{}
	for rows.Next() {
		var m domain.Message
		var id, sid, rid, role string
		if err := rows.Scan(&id, &sid, &rid, &role, &m.CreatedAt, &m.Content); err != nil {
			return nil, fmt.Errorf("storage: scan message: %w", err)
		}
		m.ID = id
		m.SessionID = domain.SessionID(sid)
		m.RunID = domain.RunID(rid)
		m.Role = domain.Role(role)
		out = append(out, m)
	}
	return out, rows.Err()
}
