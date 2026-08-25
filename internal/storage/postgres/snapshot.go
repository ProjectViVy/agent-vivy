package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"agent-vivy/internal/storage"
)

// Snapshot implements storage.SnapshotStore over the shared database
// handle.
type Snapshot struct {
	db *DB
}

// Get returns the value and version for key; absent keys yield
// (nil, 0, nil).
func (s *Snapshot) Get(ctx context.Context, key string) ([]byte, int64, error) {
	var value []byte
	var version int64
	err := s.db.QueryRowContext(ctx,
		`SELECT value, version FROM snapshots WHERE key = ?`, key).Scan(&value, &version)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, 0, nil
	}
	if err != nil {
		return nil, 0, fmt.Errorf("storage: snapshot get %q: %w", key, err)
	}
	return value, version, nil
}

// Put applies optimistic concurrency: the write succeeds only when the
// stored version equals expectVersion (0 creates the key at version 1).
func (s *Snapshot) Put(ctx context.Context, key string, value []byte, expectVersion int64) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("storage: begin snapshot put: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	var version int64
	err = tx.QueryRowContext(ctx,
		`SELECT version FROM snapshots WHERE key = ?`, key).Scan(&version)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		if expectVersion != 0 {
			return storage.ErrVersionConflict
		}
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO snapshots (key, value, version) VALUES (?, ?, 1)`,
			key, value); err != nil {
			if isUniqueConflict(err) {
				return storage.ErrVersionConflict
			}
			return fmt.Errorf("storage: snapshot insert %q: %w", key, err)
		}
	case err != nil:
		return fmt.Errorf("storage: snapshot read %q: %w", key, err)
	default:
		if version != expectVersion {
			return storage.ErrVersionConflict
		}
		res, err := tx.ExecContext(ctx,
			`UPDATE snapshots SET value = ?, version = version + 1 WHERE key = ? AND version = ?`,
			value, key, expectVersion)
		if err != nil {
			return fmt.Errorf("storage: snapshot update %q: %w", key, err)
		}
		n, err := res.RowsAffected()
		if err != nil {
			return fmt.Errorf("storage: snapshot update %q: %w", key, err)
		}
		if n == 0 {
			return storage.ErrVersionConflict
		}
	}

	return tx.Commit()
}
