package sqlite

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
	db *sql.DB
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
	return retrySQLiteBusy(ctx, func() error { return s.putCAS(ctx, key, value, expectVersion) })
}

func (s *Snapshot) putCAS(ctx context.Context, key string, value []byte, expectVersion int64) error {

	var result sql.Result
	var err error
	if expectVersion == 0 {
		result, err = s.db.ExecContext(ctx, `INSERT INTO snapshots(key,value,version) VALUES(?,?,1) ON CONFLICT(key) DO NOTHING`, key, value)
	} else {
		result, err = s.db.ExecContext(ctx, `UPDATE snapshots SET value=?,version=version+1 WHERE key=? AND version=?`, value, key, expectVersion)
	}
	if err != nil {
		return fmt.Errorf("storage: snapshot CAS %q: %w", key, err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("storage: snapshot CAS rows: %w", err)
	}
	if affected != 1 {
		return storage.ErrVersionConflict
	}
	return nil
}
