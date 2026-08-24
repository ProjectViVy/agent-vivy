package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/storage"
)

var _ storage.TodoStore = (*Backend)(nil)

func (b *Backend) CreateTodo(ctx context.Context, todo domain.Todo) error {
	blocks, blockedBy, metadata := todoJSON(todo)
	if _, err := b.db.ExecContext(ctx, `
		INSERT INTO todos (id, session_id, subject, description, status, blocks_json, blocked_by_json, active_form, owner, metadata_json, position, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`, todo.ID, todo.SessionID, todo.Subject, todo.Description, todo.Status,
		blocks, blockedBy, todo.ActiveForm, todo.Owner, metadata, todo.Position, todo.CreatedAt, todo.UpdatedAt); err != nil {
		return fmt.Errorf("storage: create todo %s: %w", todo.ID, err)
	}
	return nil
}

func (b *Backend) GetTodo(ctx context.Context, sessionID domain.SessionID, id string) (domain.Todo, error) {
	var todo domain.Todo
	var sid, status string
	var blocks, blockedBy, metadata []byte
	err := b.db.QueryRowContext(ctx, `
		SELECT id, session_id, subject, description, status, blocks_json, blocked_by_json, active_form, owner, metadata_json, position, created_at, updated_at
		FROM todos WHERE session_id = ? AND id = ?`, sessionID, id).Scan(&todo.ID, &sid, &todo.Subject, &todo.Description, &status,
		&blocks, &blockedBy, &todo.ActiveForm, &todo.Owner, &metadata, &todo.Position, &todo.CreatedAt, &todo.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Todo{}, storage.ErrNotFound
	}
	if err != nil {
		return domain.Todo{}, fmt.Errorf("storage: get todo %s: %w", id, err)
	}
	todo.SessionID = domain.SessionID(sid)
	todo.Status = domain.TodoStatus(status)
	if err := json.Unmarshal(blocks, &todo.Blocks); err != nil {
		return domain.Todo{}, fmt.Errorf("storage: decode todo blocks: %w", err)
	}
	if err := json.Unmarshal(blockedBy, &todo.BlockedBy); err != nil {
		return domain.Todo{}, fmt.Errorf("storage: decode todo blocked_by: %w", err)
	}
	todo.Metadata = append([]byte(nil), metadata...)
	return todo, nil
}

func (b *Backend) ListTodos(ctx context.Context, sessionID domain.SessionID) ([]domain.Todo, error) {
	rows, err := b.db.QueryContext(ctx, `
		SELECT id, session_id, subject, description, status, blocks_json, blocked_by_json, active_form, owner, metadata_json, position, created_at, updated_at
		FROM todos WHERE session_id = ? ORDER BY position, id`, sessionID)
	if err != nil {
		return nil, fmt.Errorf("storage: list todos: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var out []domain.Todo
	for rows.Next() {
		var todo domain.Todo
		var sid, status string
		var blocks, blockedBy, metadata []byte
		if err := rows.Scan(&todo.ID, &sid, &todo.Subject, &todo.Description, &status, &blocks, &blockedBy,
			&todo.ActiveForm, &todo.Owner, &metadata, &todo.Position, &todo.CreatedAt, &todo.UpdatedAt); err != nil {
			return nil, fmt.Errorf("storage: scan todo: %w", err)
		}
		todo.SessionID = domain.SessionID(sid)
		todo.Status = domain.TodoStatus(status)
		if err := json.Unmarshal(blocks, &todo.Blocks); err != nil {
			return nil, fmt.Errorf("storage: decode todo blocks: %w", err)
		}
		if err := json.Unmarshal(blockedBy, &todo.BlockedBy); err != nil {
			return nil, fmt.Errorf("storage: decode todo blocked_by: %w", err)
		}
		todo.Metadata = append([]byte(nil), metadata...)
		out = append(out, todo)
	}
	return out, rows.Err()
}

func (b *Backend) UpdateTodo(ctx context.Context, todo domain.Todo) error {
	blocks, blockedBy, metadata := todoJSON(todo)
	res, err := b.db.ExecContext(ctx, `
		UPDATE todos SET subject = ?, description = ?, status = ?, blocks_json = ?, blocked_by_json = ?, active_form = ?, owner = ?, metadata_json = ?, position = ?, updated_at = ?
		WHERE session_id = ? AND id = ?`, todo.Subject, todo.Description, todo.Status, blocks, blockedBy, todo.ActiveForm, todo.Owner,
		metadata, todo.Position, todo.UpdatedAt, todo.SessionID, todo.ID)
	if err != nil {
		return fmt.Errorf("storage: update todo %s: %w", todo.ID, err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("storage: update todo %s rows: %w", todo.ID, err)
	}
	if n == 0 {
		return storage.ErrNotFound
	}
	return nil
}

func todoJSON(todo domain.Todo) ([]byte, []byte, []byte) {
	blocks, _ := json.Marshal(todo.Blocks)
	blockedBy, _ := json.Marshal(todo.BlockedBy)
	metadata := todo.Metadata
	if len(metadata) == 0 {
		metadata = []byte("{}")
	}
	return blocks, blockedBy, metadata
}
