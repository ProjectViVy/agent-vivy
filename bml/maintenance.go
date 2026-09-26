// maintenance.go ports the store-maintenance surface of Diva's
// typed_store.rs: session-scoped garbage collection, the payload-free
// integrity report, VACUUM INTO backup/restore, and the canonical
// workspace-identity migration with manifest + rollback.
package bml

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

const (
	// migrationDirName holds the workspace-identity-v1 migration artifacts
	// (verified backup + manifest) beside the database, mirroring
	// {workspace_root}/.laputa/migrations/workspace-identity-v1 in Diva.
	migrationDirName = "workspace-identity-v1"
	migrationBackup  = "memory-before.sqlite3"
	migrationFile    = "manifest.json"
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

// WorkspaceIdentityMigrationState is the durable migration phase; the
// JSON spellings are serde snake_case.
type WorkspaceIdentityMigrationState string

const (
	MigrationStatePrepared   WorkspaceIdentityMigrationState = "prepared"
	MigrationStateApplied    WorkspaceIdentityMigrationState = "applied"
	MigrationStateRolledBack WorkspaceIdentityMigrationState = "rolled_back"
)

// WorkspaceIdentityMigrationManifest is the payload-free recovery record
// for the canonical workspace identity upgrade.
type WorkspaceIdentityMigrationManifest struct {
	Version              int32                           `json:"version"`
	State                WorkspaceIdentityMigrationState `json:"state"`
	LegacyWorkspaceID    string                          `json:"legacy_workspace_id"`
	CanonicalWorkspaceID string                          `json:"canonical_workspace_id"`
	BackupPath           string                          `json:"backup_path"`
	StoreRevision        int64                           `json:"store_revision"`
	RecordCount          int64                           `json:"record_count"`
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

// CanonicalWorkspaceID derives the stable workspace identity from a
// filesystem path: "workspace-" plus the first 32 hex characters of
// sha256(normalized path) (42 chars total). Port of
// agent_diva_core::workspace_identity::canonical_workspace_id.
func CanonicalWorkspaceID(path string) string {
	normalized := path
	if resolved, err := filepath.EvalSymlinks(path); err == nil {
		normalized = resolved
	} else {
		normalized = normalizeWorkspacePath(path)
	}
	sum := sha256.Sum256([]byte(normalized))
	return fmt.Sprintf("workspace-%x", sum)[:42]
}

// LegacyPathWorkspaceID is the previous typed-store identity: the raw
// path string. Exposed only so the migration layer can recognize and
// upgrade an existing store; new records must use CanonicalWorkspaceID.
func LegacyPathWorkspaceID(path string) string { return path }

// normalizeWorkspacePath is the non-IO fallback when the path cannot be
// canonicalized: absolute + clean components, separators to '/', Windows
// verbatim prefixes stripped, trailing '/' trimmed, lowercased on
// Windows.
func normalizeWorkspacePath(path string) string {
	absolute, err := filepath.Abs(path)
	if err != nil {
		absolute = path
	}
	value := strings.ReplaceAll(absolute, "\\", "/")
	if rest, ok := strings.CutPrefix(value, "//?/UNC/"); ok {
		value = "//" + rest
	} else if rest, ok := strings.CutPrefix(value, "//?/"); ok {
		value = rest
	}
	for len(value) > 1 && strings.HasSuffix(value, "/") {
		value = value[:len(value)-1]
	}
	if runtime.GOOS == "windows" {
		value = strings.ToLower(value)
	}
	return value
}

// OpenCanonical opens the store at dir under its canonical identity,
// upgrading the one recognized legacy path identity through a verified,
// content-preserving backup. When no database exists it creates a fresh
// store bound to the canonical identity.
func OpenCanonical(ctx context.Context, dir string) (*Store, error) {
	if isFile(filepath.Join(dir, storeFileName)) {
		return OpenExistingCanonical(ctx, dir)
	}
	return Open(ctx, dir, CanonicalWorkspaceID(dir))
}

// OpenExistingCanonical opens an existing store at dir under its
// canonical identity, permitting only the recognized identity-only
// legacy upgrade. Missing stores remain fail-closed.
func OpenExistingCanonical(ctx context.Context, dir string) (*Store, error) {
	path := filepath.Join(dir, storeFileName)
	if !isFile(path) {
		return nil, invalidBackupErr()
	}
	canonical := CanonicalWorkspaceID(dir)
	store, err := openPath(ctx, path, canonical)
	if err == nil {
		return store, nil
	}
	var se *StoreError
	if errors.As(err, &se) && se.Code == ErrDatabaseWorkspaceMismatch &&
		se.Actual == LegacyPathWorkspaceID(dir) {
		return migrateLegacyWorkspaceIdentity(ctx, dir, se.Actual, canonical)
	}
	return nil, err
}

// migrateLegacyWorkspaceIdentity rewrites a legacy-path-bound store to
// the canonical identity behind a verified backup + manifest.
func migrateLegacyWorkspaceIdentity(ctx context.Context, dir, legacy, canonical string) (*Store, error) {
	path := filepath.Join(dir, storeFileName)
	store, err := openPath(ctx, path, legacy)
	if err != nil {
		return nil, err
	}
	migrationDir := filepath.Join(dir, "migrations", migrationDirName)
	if err := os.MkdirAll(migrationDir, 0o755); err != nil {
		_ = store.Close()
		return nil, ioErr(migrationDir, err)
	}
	backup := filepath.Join(migrationDir, migrationBackup)
	manifestPath := filepath.Join(migrationDir, migrationFile)
	if !isFile(backup) {
		if err := store.Backup(ctx, backup); err != nil {
			_ = store.Close()
			return nil, err
		}
	} else {
		// An earlier attempt already left a backup; it must still validate
		// as a store bound to the legacy identity.
		validation, err := openPath(ctx, backup, legacy)
		if err != nil {
			_ = store.Close()
			return nil, err
		}
		_ = validation.Close()
	}
	integrity, err := store.Integrity(ctx)
	if err != nil {
		_ = store.Close()
		return nil, err
	}
	manifest := WorkspaceIdentityMigrationManifest{
		Version:              1,
		State:                MigrationStatePrepared,
		LegacyWorkspaceID:    legacy,
		CanonicalWorkspaceID: canonical,
		BackupPath:           backup,
		StoreRevision:        integrity.StoreRevision,
		RecordCount:          integrity.RecordCount,
	}
	if err := atomicWriteJSON(manifestPath, &manifest); err != nil {
		_ = store.Close()
		return nil, identityManifestErr(err)
	}

	tx, err := store.db.BeginTx(ctx, nil)
	if err != nil {
		_ = store.Close()
		return nil, persistenceErr(store.path, err)
	}
	defer tx.Rollback()
	rows, err := tx.QueryContext(ctx,
		"SELECT memory_id, record_json FROM memory_records")
	if err != nil {
		_ = store.Close()
		return nil, persistenceErr(store.path, err)
	}
	type migrationRow struct {
		id   string
		json string
	}
	var pending []migrationRow
	for rows.Next() {
		var row migrationRow
		if err := rows.Scan(&row.id, &row.json); err != nil {
			rows.Close()
			_ = store.Close()
			return nil, persistenceErr(store.path, err)
		}
		pending = append(pending, row)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		_ = store.Close()
		return nil, persistenceErr(store.path, err)
	}
	rows.Close()
	for _, row := range pending {
		var record Record
		if err := json.Unmarshal([]byte(row.json), &record); err != nil {
			_ = store.Close()
			return nil, corruptRecordErr()
		}
		if record.Scope.WorkspaceID != legacy {
			_ = store.Close()
			return nil, identityMigrationRejectedErr()
		}
		record.Scope.WorkspaceID = canonical
		migratedJSON, err := marshalCanonical(record)
		if err != nil {
			_ = store.Close()
			return nil, corruptRecordErr()
		}
		if _, err := tx.ExecContext(ctx,
			"UPDATE memory_records SET workspace_id = ?, record_json = ? WHERE memory_id = ?",
			canonical, string(migratedJSON), row.id); err != nil {
			_ = store.Close()
			return nil, persistenceErr(store.path, err)
		}
	}
	if _, err := tx.ExecContext(ctx,
		"UPDATE memory_fts SET workspace_id = ?", canonical); err != nil {
		_ = store.Close()
		return nil, persistenceErr(store.path, err)
	}
	if _, err := tx.ExecContext(ctx,
		"UPDATE schema_meta SET workspace_id = ? WHERE component = ?",
		canonical, component); err != nil {
		_ = store.Close()
		return nil, persistenceErr(store.path, err)
	}
	if err := tx.Commit(); err != nil {
		_ = store.Close()
		return nil, persistenceErr(store.path, err)
	}
	_ = store.Close()

	migrated, err := openPath(ctx, path, canonical)
	if err != nil {
		return nil, err
	}
	migratedIntegrity, err := migrated.Integrity(ctx)
	if err != nil {
		_ = migrated.Close()
		return nil, err
	}
	if migratedIntegrity.StoreRevision != manifest.StoreRevision ||
		migratedIntegrity.RecordCount != manifest.RecordCount ||
		len(migratedIntegrity.CorruptRecordIDs) != 0 ||
		migratedIntegrity.OrphanFTSRows != 0 {
		_ = migrated.Close()
		return nil, corruptRecordErr()
	}
	manifest.State = MigrationStateApplied
	if err := atomicWriteJSON(manifestPath, &manifest); err != nil {
		_ = migrated.Close()
		return nil, identityManifestErr(err)
	}
	return migrated, nil
}

// RollbackCanonicalIdentity restores the verified pre-migration database
// recorded in the manifest. It rejects unless the manifest is version 1,
// state applied, and both identities and the backup path match dir.
func RollbackCanonicalIdentity(ctx context.Context, dir string) (WorkspaceIdentityMigrationManifest, error) {
	migrationDir := filepath.Join(dir, "migrations", migrationDirName)
	manifestPath := filepath.Join(migrationDir, migrationFile)
	raw, err := os.ReadFile(manifestPath)
	if err != nil {
		return WorkspaceIdentityMigrationManifest{}, ioErr(manifestPath, err)
	}
	var manifest WorkspaceIdentityMigrationManifest
	if err := json.Unmarshal(raw, &manifest); err != nil {
		return WorkspaceIdentityMigrationManifest{}, identityManifestErr(err)
	}
	if manifest.Version != 1 ||
		manifest.State != MigrationStateApplied ||
		manifest.CanonicalWorkspaceID != CanonicalWorkspaceID(dir) ||
		manifest.LegacyWorkspaceID != LegacyPathWorkspaceID(dir) ||
		manifest.BackupPath != filepath.Join(migrationDir, migrationBackup) {
		return WorkspaceIdentityMigrationManifest{}, identityMigrationRejectedErr()
	}
	database := filepath.Join(dir, storeFileName)
	current, err := openPath(ctx, database, manifest.CanonicalWorkspaceID)
	if err != nil {
		return WorkspaceIdentityMigrationManifest{}, err
	}
	currentIntegrity, err := current.Integrity(ctx)
	if err != nil {
		_ = current.Close()
		return WorkspaceIdentityMigrationManifest{}, err
	}
	if currentIntegrity.StoreRevision != manifest.StoreRevision ||
		currentIntegrity.RecordCount != manifest.RecordCount {
		_ = current.Close()
		return WorkspaceIdentityMigrationManifest{}, identityMigrationRejectedErr()
	}
	_ = current.Close()

	backupValidation, err := openPath(ctx, manifest.BackupPath, manifest.LegacyWorkspaceID)
	if err != nil {
		return WorkspaceIdentityMigrationManifest{}, err
	}
	backupIntegrity, err := backupValidation.Integrity(ctx)
	if err != nil {
		_ = backupValidation.Close()
		return WorkspaceIdentityMigrationManifest{}, err
	}
	_ = backupValidation.Close()
	if backupIntegrity.StoreRevision != manifest.StoreRevision ||
		backupIntegrity.RecordCount != manifest.RecordCount ||
		len(backupIntegrity.CorruptRecordIDs) != 0 {
		return WorkspaceIdentityMigrationManifest{}, invalidBackupErr()
	}
	if err := copyFile(manifest.BackupPath, database); err != nil {
		return WorkspaceIdentityMigrationManifest{}, ioErr(database, err)
	}
	manifest.State = MigrationStateRolledBack
	if err := atomicWriteJSON(manifestPath, &manifest); err != nil {
		return WorkspaceIdentityMigrationManifest{}, identityManifestErr(err)
	}
	return manifest, nil
}

// openPath opens (or initializes) a store at an explicit database file
// path — the Rust open_path used by migration and restore, which open
// files outside the {dir}/memory.sqlite3 convention.
func openPath(ctx context.Context, path, workspaceID string) (*Store, error) {
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

func identityMigrationRejectedErr() *StoreError {
	return &StoreError{
		Code:    ErrIdentityMigrationRejected,
		Message: "workspace identity migration only accepts the recognized legacy path identity",
	}
}

func identityManifestErr(err error) *StoreError {
	return &StoreError{
		Code:    ErrIdentityManifest,
		Message: fmt.Sprintf("workspace identity migration manifest failed: %v", err),
		Err:     err,
	}
}
