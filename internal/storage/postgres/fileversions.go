package postgres

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/storage"
)

var _ storage.FileVersionStore = (*Backend)(nil)

// RecordFileMutation appends one mutation onto the (session, path) version
// chain in a single transaction. See storage.FileVersionStore for the
// semantics. Recording is best-effort upstream: callers log failures and
// never block the mutation itself.
func (b *Backend) RecordFileMutation(ctx context.Context, sessionID domain.SessionID, runID domain.RunID, path string, oldContent, newContent []byte) error {
	tx, err := b.db.SQL.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("storage: begin file version: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	var latestVersion int64
	var latestContent []byte
	err = tx.QueryRowContext(ctx, `
		SELECT version, content FROM file_versions
		WHERE session_id = $1 AND path = $2 ORDER BY version DESC LIMIT 1`,
		sessionID, path).Scan(&latestVersion, &latestContent)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		latestVersion, latestContent = 0, nil
	case err != nil:
		return fmt.Errorf("storage: read file version head %s: %w", path, err)
	}

	// Archive the pre-mutation content: as the baseline on first sight, as
	// an intermediate state when the file moved under the chain.
	next := latestVersion
	latestAfterOld := latestContent
	if latestVersion == 0 || !bytes.Equal(latestContent, oldContent) {
		if len(oldContent) <= storage.FileVersionMaxBytes {
			next++
			if err := insertFileVersion(ctx, tx, sessionID, runID, path, next, oldContent); err != nil {
				return err
			}
			latestAfterOld = oldContent
		}
	}
	if len(newContent) <= storage.FileVersionMaxBytes && (next == 0 || !bytes.Equal(newContent, latestAfterOld)) {
		next++
		if err := insertFileVersion(ctx, tx, sessionID, runID, path, next, newContent); err != nil {
			return err
		}
	}

	// Keep only the newest FileVersionRetention versions per path (O2).
	var cutoff sql.NullInt64
	if err := tx.QueryRowContext(ctx, `
		SELECT version FROM file_versions
		WHERE session_id = $1 AND path = $2 ORDER BY version DESC LIMIT 1 OFFSET $3`,
		sessionID, path, storage.FileVersionRetention).Scan(&cutoff); err != nil && !errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("storage: file version cutoff %s: %w", path, err)
	}
	if cutoff.Valid {
		if _, err := tx.ExecContext(ctx, `
			DELETE FROM file_versions WHERE session_id = $1 AND path = $2 AND version <= $3`,
			sessionID, path, cutoff.Int64); err != nil {
			return fmt.Errorf("storage: prune file versions %s: %w", path, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("storage: commit file version: %w", err)
	}
	return nil
}

func insertFileVersion(ctx context.Context, tx *sql.Tx, sessionID domain.SessionID, runID domain.RunID, path string, version int64, content []byte) error {
	// A brand-new file's pre-mutation content arrives as nil; storing NULL
	// would violate the NOT NULL constraint on file_versions.content.
	if content == nil {
		content = []byte{}
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO file_versions (session_id, run_id, path, version, content, created_at)
		VALUES ($1, $2, $3, $4, $5, $6)`,
		sessionID, runID, path, version, content, time.Now().UnixMilli()); err != nil {
		return fmt.Errorf("storage: insert file version %s@%d: %w", path, version, err)
	}
	return nil
}

// TrackFileAccess records bounded actual bytes, with absence distinct from empty.
func (b *Backend) TrackFileAccess(ctx context.Context, sessionID domain.SessionID, path string, state domain.FileReadState) error {
	if len(state.Content) > storage.FileVersionMaxBytes || (!state.Exists && len(state.Content) != 0) {
		return fmt.Errorf("storage: invalid file read state")
	}
	content := state.Content
	if content == nil {
		content = []byte{}
	}
	exists := 0
	if state.Exists {
		exists = 1
	}
	if _, err := b.db.ExecContext(ctx, `
 INSERT INTO file_reads(session_id,path,read_at,read_exists,read_content) VALUES(?,?,?,?,?)
 ON CONFLICT(session_id,path) DO UPDATE SET read_at=excluded.read_at,read_exists=excluded.read_exists,read_content=excluded.read_content`, sessionID, path, time.Now().UnixMilli(), exists, content); err != nil {
		return fmt.Errorf("storage: track file access: %w", err)
	}
	return nil
}

// LastFileAccess distinguishes never tracked from historical unavailable bytes.
func (b *Backend) LastFileAccess(ctx context.Context, sessionID domain.SessionID, path string) (*domain.FileReadState, bool, error) {
	var exists sql.NullInt64
	var content []byte
	var available bool
	err := b.db.QueryRowContext(ctx, `SELECT read_exists,read_content,read_content IS NOT NULL FROM file_reads WHERE session_id=? AND path=?`, sessionID, path).Scan(&exists, &content, &available)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, fmt.Errorf("storage: last file access: %w", err)
	}
	if !exists.Valid || !available || (exists.Int64 != 0 && exists.Int64 != 1) || len(content) > storage.FileVersionMaxBytes || (exists.Int64 == 0 && len(content) > 0) {
		return nil, true, nil
	}
	if content == nil {
		content = []byte{}
	}
	return &domain.FileReadState{Exists: exists.Int64 == 1, Content: content}, true, nil
}
