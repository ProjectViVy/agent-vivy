// maintenance.go ports the store-maintenance surface of Diva's
// typed_store.rs: session-scoped garbage collection, the payload-free
// integrity report, VACUUM INTO backup/restore, and the canonical
// workspace-identity migration with manifest + rollback.
package bml

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// StoreIntegrity is the payload-free integrity summary
// (MemoryStoreIntegrity in typed_store.rs). IntegrityReport remains the
// migration-comparison type from memory/record.rs.
type StoreIntegrity struct {
	SchemaVersion       int64    `json:"schema_version"`
	StoreRevision       int64    `json:"store_revision"`
	RecordCount         int64    `json:"record_count"`
	TombstoneCount      int64    `json:"tombstone_count"`
	ContentBytes        int64    `json:"content_bytes"`
	FTSRowCount         int64    `json:"fts_row_count"`
	SupersedesEdgeCount int64    `json:"supersedes_edge_count"`
	CorruptRecordIDs    []string `json:"corrupt_record_ids"`
	OrphanFTSRows       int64    `json:"orphan_fts_rows"`
}

// GCSessionScoped physically deletes every record scoped to sessionID,
// along with its FTS rows and outgoing supersedes edges. Records with a
// NULL session_id (long-term authority) are never touched. Faithful to
// gc_session_scoped: schema_meta counters are intentionally not updated.
func (s *Store) GCSessionScoped(ctx context.Context, sessionID string) (uint64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, persistenceErr(s.path, err)
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx,
		`DELETE FROM memory_supersedes WHERE memory_id IN
		   (SELECT memory_id FROM memory_records WHERE session_id = ?)`,
		sessionID); err != nil {
		return 0, persistenceErr(s.path, err)
	}
	if _, err := tx.ExecContext(ctx,
		`DELETE FROM memory_fts WHERE session_id = ?`, sessionID); err != nil {
		return 0, persistenceErr(s.path, err)
	}
	res, err := tx.ExecContext(ctx,
		`DELETE FROM memory_records WHERE session_id = ?`, sessionID)
	if err != nil {
		return 0, persistenceErr(s.path, err)
	}
	if err := tx.Commit(); err != nil {
		return 0, persistenceErr(s.path, err)
	}
	rows, err := res.RowsAffected()
	if err != nil {
		return 0, persistenceErr(s.path, err)
	}
	return uint64(rows), nil
}

// GCStaleSessionScoped is the startup sweep: it physically deletes every
// session-scoped record whose session_id is NOT in activeSessionIDs.
// An empty list means "no active sessions" and removes every
// session-scoped record; NULL session_id rows are never touched.
func (s *Store) GCStaleSessionScoped(ctx context.Context, activeSessionIDs []string) (uint64, error) {
	clause := "session_id IS NOT NULL"
	if len(activeSessionIDs) > 0 {
		placeholders := make([]string, len(activeSessionIDs))
		for i := range placeholders {
			placeholders[i] = "?"
		}
		clause += " AND session_id NOT IN (" + strings.Join(placeholders, ", ") + ")"
	}
	args := make([]any, len(activeSessionIDs))
	for i, id := range activeSessionIDs {
		args[i] = id
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, persistenceErr(s.path, err)
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx,
		`DELETE FROM memory_supersedes WHERE memory_id IN
		   (SELECT memory_id FROM memory_records WHERE `+clause+`)`,
		args...); err != nil {
		return 0, persistenceErr(s.path, err)
	}
	if _, err := tx.ExecContext(ctx,
		`DELETE FROM memory_fts WHERE `+clause, args...); err != nil {
		return 0, persistenceErr(s.path, err)
	}
	res, err := tx.ExecContext(ctx,
		`DELETE FROM memory_records WHERE `+clause, args...)
	if err != nil {
		return 0, persistenceErr(s.path, err)
	}
	if err := tx.Commit(); err != nil {
		return 0, persistenceErr(s.path, err)
	}
	rows, err := res.RowsAffected()
	if err != nil {
		return 0, persistenceErr(s.path, err)
	}
	return uint64(rows), nil
}

// Integrity returns the payload-free store summary. A row is corrupt when
// its record_json fails to decode, or when the decoded record disagrees
// with the row's id/workspace/tombstone/content_bytes, or when it fails
// validation against now (5-minute skew) or this store's workspace.
func (s *Store) Integrity(ctx context.Context) (StoreIntegrity, error) {
	metadata, err := s.Metadata(ctx)
	if err != nil {
		return StoreIntegrity{}, err
	}
	scalar := func(query string) (int64, error) {
		var n int64
		if err := s.db.QueryRowContext(ctx, query).Scan(&n); err != nil {
			return 0, persistenceErr(s.path, err)
		}
		return n, nil
	}

	var report StoreIntegrity
	report.SchemaVersion = metadata.SchemaVersion
	report.StoreRevision = metadata.StoreRevision
	for _, target := range []struct {
		query string
		dst   *int64
	}{
		{"SELECT COUNT(*) FROM memory_records", &report.RecordCount},
		{"SELECT COUNT(*) FROM memory_records WHERE tombstone = 1", &report.TombstoneCount},
		{"SELECT COALESCE(SUM(content_bytes), 0) FROM memory_records", &report.ContentBytes},
		{"SELECT COUNT(*) FROM memory_fts", &report.FTSRowCount},
		{"SELECT COUNT(*) FROM memory_supersedes", &report.SupersedesEdgeCount},
		{`SELECT COUNT(*) FROM memory_fts f
		    LEFT JOIN memory_records r ON r.memory_id = f.memory_id
		    WHERE r.memory_id IS NULL OR r.tombstone = 1`, &report.OrphanFTSRows},
	} {
		if *target.dst, err = scalar(target.query); err != nil {
			return StoreIntegrity{}, err
		}
	}

	rows, err := s.db.QueryContext(ctx,
		`SELECT memory_id, workspace_id, tombstone, content_bytes, record_json
		 FROM memory_records ORDER BY memory_id`)
	if err != nil {
		return StoreIntegrity{}, persistenceErr(s.path, err)
	}
	defer rows.Close()
	now := time.Now()
	for rows.Next() {
		var id, workspace, recordJSON string
		var tombstone, contentBytes int64
		if err := rows.Scan(&id, &workspace, &tombstone, &contentBytes, &recordJSON); err != nil {
			return StoreIntegrity{}, persistenceErr(s.path, err)
		}
		var record Record
		valid := json.Unmarshal([]byte(recordJSON), &record) == nil &&
			record.ID == id &&
			record.Scope.WorkspaceID == workspace &&
			boolInt(record.Tombstone != nil) == tombstone &&
			int64(len(record.Content)) == contentBytes &&
			record.ValidateAt(now, validationClockSkew) == nil &&
			record.ValidateWorkspace(s.workspaceID) == nil
		if !valid {
			report.CorruptRecordIDs = append(report.CorruptRecordIDs, id)
		}
	}
	if err := rows.Err(); err != nil {
		return StoreIntegrity{}, persistenceErr(s.path, err)
	}
	return report, nil
}

// Backup writes a transactionally consistent snapshot of the database to
// destination via VACUUM INTO. It fails with ErrBackupExists when
// destination already exists.
func (s *Store) Backup(ctx context.Context, destination string) error {
	if _, err := os.Stat(destination); err == nil {
		return backupExistsErr(destination)
	} else if !os.IsNotExist(err) {
		return ioErr(destination, err)
	}
	escaped := strings.ReplaceAll(destination, "'", "''")
	if _, err := s.db.ExecContext(ctx, fmt.Sprintf("VACUUM INTO '%s'", escaped)); err != nil {
		return persistenceErr(s.path, err)
	}
	return nil
}

// Restore closes this store, validates backup as a store bound to the
// same workspace, replaces the database file, and reopens. It mirrors
// Rust `restore(self)`: the receiver is closed on every path, success or
// failure.
func (s *Store) Restore(ctx context.Context, backup string) (*Store, error) {
	info, err := os.Stat(backup)
	if err != nil || info.IsDir() {
		_ = s.Close()
		return nil, invalidBackupErr()
	}
	valid, err := openPath(ctx, backup, s.workspaceID)
	if err != nil {
		_ = s.Close()
		return nil, invalidBackupErr()
	}
	_ = valid.Close()
	_ = s.Close()
	if err := copyFile(backup, s.path); err != nil {
		return nil, ioErr(s.path, err)
	}
	return openPath(ctx, s.path, s.workspaceID)
}

// OpenCanonical opens the persisted workspace owner or allocates one for a fresh home.
func OpenCanonical(ctx context.Context, dir string) (*Store, error) {
	path := filepath.Join(dir, storeFileName)
	if _, err := os.Stat(path); err == nil {
		return OpenExistingCanonical(ctx, dir)
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, ioErr(path, err)
	}
	return Open(ctx, dir, opaqueID("workspace"))
}

// OpenExistingCanonical reads the owner from the validated store. Relocation
// preserves that owner; callers requiring a particular owner use OpenExisting.
func OpenExistingCanonical(ctx context.Context, dir string) (*Store, error) {
	path := filepath.Join(dir, storeFileName)
	owner, _, err := inspectStore(ctx, path, "")
	if err != nil {
		return nil, err
	}
	if owner == "" {
		return nil, invalidBackupErr()
	}
	return openPath(ctx, path, owner)
}

// openPath opens (or initializes) a store at an explicit database file
// path — the Rust open_path used by migration and restore, which open
// files outside the {dir}/memory.sqlite3 convention.
func openPath(ctx context.Context, path, workspaceID string) (*Store, error) {
	if _, _, err := inspectStore(ctx, path, workspaceID); err != nil {
		return nil, err
	}
	if parent := filepath.Dir(path); parent != "" && parent != "." {
		if err := os.MkdirAll(parent, 0o755); err != nil {
			return nil, ioErr(parent, err)
		}
	}
	db, err := sql.Open("sqlite", fmt.Sprintf("file:%s?_pragma=journal_mode(WAL)&_pragma=foreign_keys(1)&_pragma=busy_timeout(%d)", path, busyTimeoutMS))
	if err != nil {
		return nil, ioErr(path, err)
	}
	db.SetMaxOpenConns(4)
	s := &Store{db: db, path: path, workspaceID: workspaceID}
	if err := s.initialize(ctx); err != nil {
		_ = db.Close()
		return nil, err
	}
	return s, nil
}

func isFile(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o644)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		_ = out.Close()
		return err
	}
	return out.Close()
}

func backupExistsErr(path string) *StoreError {
	return &StoreError{
		Code:    ErrBackupExists,
		Message: fmt.Sprintf("backup path already exists: %s", path),
	}
}

func invalidBackupErr() *StoreError {
	return &StoreError{
		Code:    ErrInvalidBackup,
		Message: "backup is not a valid Embedded Laputa database",
	}
}
