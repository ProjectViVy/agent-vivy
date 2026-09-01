package postgres

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
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
	var latestHash string
	err = tx.QueryRowContext(ctx, `
		SELECT version, content_hash FROM file_versions
		WHERE session_id = $1 AND path = $2 ORDER BY version DESC LIMIT 1`,
		sessionID, path).Scan(&latestVersion, &latestHash)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		latestVersion, latestHash = 0, ""
	case err != nil:
		return fmt.Errorf("storage: read file version head %s: %w", path, err)
	}

	// Archive the pre-mutation content: as the baseline on first sight, as
	// an intermediate state when the file moved under the chain.
	next := latestVersion
	latestAfterOld := latestHash
	if latestVersion == 0 || latestHash != fileContentHash(oldContent) {
		if len(oldContent) <= storage.FileVersionMaxBytes {
			next++
			if err := insertFileVersion(ctx, tx, sessionID, runID, path, next, oldContent); err != nil {
				return err
			}
			latestAfterOld = fileContentHash(oldContent)
		}
	}
	if newHash := fileContentHash(newContent); len(newContent) <= storage.FileVersionMaxBytes && newHash != latestAfterOld {
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
		INSERT INTO file_versions (session_id, run_id, path, version, content_hash, content, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		sessionID, runID, path, version, fileContentHash(content), content, time.Now().UnixMilli()); err != nil {
		return fmt.Errorf("storage: insert file version %s@%d: %w", path, version, err)
	}
	return nil
}

// TrackFileAccess upserts the stale-read marker (the filetracker).
func (b *Backend) TrackFileAccess(ctx context.Context, sessionID domain.SessionID, path string, at int64) error {
	if _, err := b.db.ExecContext(ctx, `
		INSERT INTO file_reads (session_id, path, read_at) VALUES (?, ?, ?)
		ON CONFLICT(session_id, path) DO UPDATE SET read_at = excluded.read_at`,
		sessionID, path, at); err != nil {
		return fmt.Errorf("storage: track file access %s: %w", path, err)
	}
	return nil
}

// LastFileAccess returns the marker timestamp; ok=false when never tracked.
func (b *Backend) LastFileAccess(ctx context.Context, sessionID domain.SessionID, path string) (int64, bool, error) {
	var at int64
	err := b.db.QueryRowContext(ctx,
		`SELECT read_at FROM file_reads WHERE session_id = ? AND path = ?`, sessionID, path).Scan(&at)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, fmt.Errorf("storage: last file access %s: %w", path, err)
	}
	return at, true, nil
}

func fileContentHash(content []byte) string {
	sum := sha256.Sum256(content)
	return hex.EncodeToString(sum[:])
}
