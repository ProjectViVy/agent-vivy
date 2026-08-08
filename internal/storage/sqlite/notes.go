package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/storage"
)

// AppendNote inserts one append-only notebook entry (MA-3); there is no
// update path, mirroring the message log (FR-2).
func (b *Backend) AppendNote(ctx context.Context, n domain.Note) error {
	if _, err := b.db.ExecContext(ctx,
		`INSERT INTO notes (id, content, created_at) VALUES (?, ?, ?)`,
		n.ID, n.Content, n.CreatedAt); err != nil {
		return fmt.Errorf("storage: append note %s: %w", n.ID, err)
	}
	return nil
}

// ListNotes returns the whole notebook newest-first, so bounded digests
// can read the head. An empty notebook yields an empty list.
func (b *Backend) ListNotes(ctx context.Context) ([]domain.Note, error) {
	rows, err := b.db.QueryContext(ctx,
		`SELECT id, content, created_at FROM notes ORDER BY created_at DESC, id DESC`)
	if err != nil {
		return nil, fmt.Errorf("storage: list notes: %w", err)
	}
	defer func() { _ = rows.Close() }()

	out := []domain.Note{}
	for rows.Next() {
		var n domain.Note
		if err := rows.Scan(&n.ID, &n.Content, &n.CreatedAt); err != nil {
			return nil, fmt.Errorf("storage: scan note: %w", err)
		}
		out = append(out, n)
	}
	return out, rows.Err()
}

// GetNote fetches one entry by id; absent ids yield storage.ErrNotFound.
func (b *Backend) GetNote(ctx context.Context, id string) (domain.Note, error) {
	var n domain.Note
	err := b.db.QueryRowContext(ctx,
		`SELECT id, content, created_at FROM notes WHERE id = ?`, id).
		Scan(&n.ID, &n.Content, &n.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Note{}, storage.ErrNotFound
	}
	if err != nil {
		return domain.Note{}, fmt.Errorf("storage: get note %s: %w", id, err)
	}
	return n, nil
}
