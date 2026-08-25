package postgres

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"time"
)

// Blobs implements storage.BlobStore over the shared database handle.
type Blobs struct {
	db *DB
}

// Put appends a new generation for id and atomically flips the pointer
// row (D-030). Existing generations are never mutated in place.
func (s *Blobs) Put(ctx context.Context, id string, data []byte) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("storage: begin blob put: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	var gen int64
	if err := tx.QueryRowContext(ctx,
		`SELECT COALESCE(MAX(generation), 0) FROM checkpoint_generations WHERE id = ?`,
		id).Scan(&gen); err != nil {
		return fmt.Errorf("storage: read generation %q: %w", id, err)
	}
	gen++

	if _, err := tx.ExecContext(ctx,
		`INSERT INTO checkpoint_generations (id, generation, blob) VALUES (?, ?, ?)`,
		id, gen, data); err != nil {
		return fmt.Errorf("storage: insert generation %q/%d: %w", id, gen, err)
	}

	sum := sha256.Sum256(data)
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO checkpoints (id, generation, checksum, created_at) VALUES (?, ?, ?, ?)
		 ON CONFLICT(id) DO UPDATE SET
			generation = excluded.generation,
			checksum = excluded.checksum,
			created_at = excluded.created_at`,
		id, gen, hex.EncodeToString(sum[:]), time.Now().UnixMilli()); err != nil {
		return fmt.Errorf("storage: flip pointer %q: %w", id, err)
	}

	return tx.Commit()
}

// Get returns the current generation's blob; absent ids yield
// (nil, false, nil).
func (s *Blobs) Get(ctx context.Context, id string) ([]byte, bool, error) {
	var data []byte
	err := s.db.QueryRowContext(ctx,
		`SELECT cg.blob FROM checkpoints c
		 JOIN checkpoint_generations cg
		   ON cg.id = c.id AND cg.generation = c.generation
		 WHERE c.id = ?`, id).Scan(&data)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, fmt.Errorf("storage: blob get %q: %w", id, err)
	}
	return data, true, nil
}

// Delete removes the pointer row and every stored generation.
func (s *Blobs) Delete(ctx context.Context, id string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("storage: begin blob delete: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.ExecContext(ctx, `DELETE FROM checkpoints WHERE id = ?`, id); err != nil {
		return fmt.Errorf("storage: delete pointer %q: %w", id, err)
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM checkpoint_generations WHERE id = ?`, id); err != nil {
		return fmt.Errorf("storage: delete generations %q: %w", id, err)
	}
	return tx.Commit()
}
