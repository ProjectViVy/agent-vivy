package bml

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	_ "modernc.org/sqlite"
)

const (
	schemaVersion int64 = 1
	component           = "embedded_laputa"
	storeFileName       = "memory.sqlite3"
	busyTimeoutMS       = 5000
)

// MaxMemoryRecords is the initial bounded-store record capacity.
const MaxMemoryRecords int64 = 10_000

// MaxMemoryContentBytes is the initial bounded-store aggregate canonical
// content capacity.
const MaxMemoryContentBytes int64 = 32 * 1024 * 1024

// StoreErrorCode is a stable, machine-matchable failure code carrying the
// snake_case TypedMemoryStoreError variant name from the Rust contract.
type StoreErrorCode string

const (
	ErrInvalidRecord             StoreErrorCode = "invalid_record"
	ErrWorkspaceMismatch         StoreErrorCode = "workspace_mismatch"
	ErrDatabaseWorkspaceMismatch StoreErrorCode = "database_workspace_mismatch"
	ErrUnsupportedSchema         StoreErrorCode = "unsupported_schema"
	ErrStoreRevisionConflict     StoreErrorCode = "store_revision_conflict"
	ErrRecordRevisionConflict    StoreErrorCode = "record_revision_conflict"
	ErrCapacityExceeded          StoreErrorCode = "capacity_exceeded"
	ErrFTSUnavailable            StoreErrorCode = "fts_unavailable"
	ErrIO                        StoreErrorCode = "io_error"
)

// StoreError is a stable typed-store failure. Message never contains Memory
// record content; Err carries the wrapped cause when one exists.
type StoreError struct {
	Code    StoreErrorCode
	Message string
	Err     error
}

func (e *StoreError) Error() string { return e.Message }
func (e *StoreError) Unwrap() error { return e.Err }

func invalidRecordErr(err *ValidationError) *StoreError {
	return &StoreError{
		Code:    ErrInvalidRecord,
		Message: "typed Memory record is invalid: " + err.Error(),
		Err:     err,
	}
}

func workspaceMismatchErr(expected, actual string) *StoreError {
	return &StoreError{
		Code:    ErrWorkspaceMismatch,
		Message: fmt.Sprintf("record belongs to workspace %s, expected %s", actual, expected),
	}
}

func databaseWorkspaceMismatchErr(expected, actual string) *StoreError {
	return &StoreError{
		Code:    ErrDatabaseWorkspaceMismatch,
		Message: fmt.Sprintf("stored database belongs to workspace %s, expected %s", actual, expected),
	}
}

func unsupportedSchemaErr(actual int64) *StoreError {
	return &StoreError{
		Code:    ErrUnsupportedSchema,
		Message: fmt.Sprintf("unsupported Embedded Laputa schema version %d", actual),
	}
}

func storeRevisionConflictErr(expected, actual int64) *StoreError {
	return &StoreError{
		Code:    ErrStoreRevisionConflict,
		Message: fmt.Sprintf("Embedded Laputa store revision conflict: expected %d, actual %d", expected, actual),
	}
}

func recordRevisionConflictErr(recordID string, expected, actual *int64) *StoreError {
	fmtOpt := func(v *int64) string {
		if v == nil {
			return "None"
		}
		return fmt.Sprintf("Some(%d)", *v)
	}
	return &StoreError{
		Code:    ErrRecordRevisionConflict,
		Message: fmt.Sprintf("Memory record revision conflict for %s: expected %s, actual %s", recordID, fmtOpt(expected), fmtOpt(actual)),
	}
}

func capacityExceededErr(records, contentBytes int64) *StoreError {
	return &StoreError{
		Code:    ErrCapacityExceeded,
		Message: fmt.Sprintf("Embedded Laputa capacity exceeded: %d records, %d content bytes", records, contentBytes),
	}
}

func ftsUnavailableErr(err error) *StoreError {
	return &StoreError{
		Code:    ErrFTSUnavailable,
		Message: "FTS5 is unavailable in the active SQLite runtime",
		Err:     err,
	}
}

func ioErr(path string, err error) *StoreError {
	return &StoreError{
		Code:    ErrIO,
		Message: fmt.Sprintf("Embedded Laputa filesystem operation failed at %s: %v", path, err),
		Err:     err,
	}
}

// persistenceErr maps SQLite failures onto io_error, the single
// environmental-error code kept from the Rust contract.
func persistenceErr(path string, err error) *StoreError {
	return &StoreError{
		Code:    ErrIO,
		Message: fmt.Sprintf("Embedded Laputa persistence failed at %s: %v", path, err),
		Err:     err,
	}
}

// StoreMetadata is the current database identity and optimistic concurrency
// revision plus the bounded-store counters maintained by schema_meta.
type StoreMetadata struct {
	SchemaVersion int64  `json:"schema_version"`
	StoreRevision int64  `json:"store_revision"`
	RecordCount   int64  `json:"record_count"`
	ContentBytes  int64  `json:"content_bytes"`
	WorkspaceID   string `json:"workspace_id"`
}

// Store is a store scoped to exactly one workspace, backed by a single
// SQLite database file. All read-modify-write transactions serialize through
// mu, mirroring the Rust write_lock.
type Store struct {
	db          *sql.DB
	path        string
	workspaceID string
	mu          sync.Mutex
}

// Open creates {dir}/memory.sqlite3 when absent and initializes the schema,
// or validates the identity of the existing database. dir is created when
// missing.
func Open(ctx context.Context, dir, workspaceID string) (*Store, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, ioErr(dir, err)
	}
	path := filepath.Join(dir, storeFileName)
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

// OpenExisting opens the store at {dir}/memory.sqlite3 read-only, without
// creating or modifying filesystem state. A missing database fails closed.
func OpenExisting(ctx context.Context, dir, workspaceID string) (*Store, error) {
	path := filepath.Join(dir, storeFileName)
	info, err := os.Stat(path)
	if err != nil || info.IsDir() {
		if err == nil {
			err = fmt.Errorf("not a file: %s", path)
		}
		return nil, ioErr(path, err)
	}
	db, err := sql.Open("sqlite", fmt.Sprintf("file:%s?mode=ro&_pragma=foreign_keys(1)&_pragma=busy_timeout(%d)", path, busyTimeoutMS))
	if err != nil {
		return nil, ioErr(path, err)
	}
	db.SetMaxOpenConns(1)
	s := &Store{db: db, path: path, workspaceID: workspaceID}
	if err := s.checkIdentity(ctx); err != nil {
		_ = db.Close()
		return nil, err
	}
	return s, nil
}

// Path returns the database file path.
func (s *Store) Path() string { return s.path }

// Close releases all SQLite connections.
func (s *Store) Close() error { return s.db.Close() }

// Metadata returns the current database identity and bounded-store counters.
func (s *Store) Metadata(ctx context.Context) (StoreMetadata, error) {
	var m StoreMetadata
	err := s.db.QueryRowContext(ctx, selectMetadataSQL, component).
		Scan(&m.SchemaVersion, &m.StoreRevision, &m.RecordCount, &m.ContentBytes, &m.WorkspaceID)
	if err != nil {
		return StoreMetadata{}, persistenceErr(s.path, err)
	}
	return m, nil
}

// schemaDDL is byte-faithful to typed_store.rs initialize().
var schemaDDL = []string{
	`CREATE TABLE IF NOT EXISTS schema_meta (
               component TEXT PRIMARY KEY,
               schema_version INTEGER NOT NULL,
               store_revision INTEGER NOT NULL,
               record_count INTEGER NOT NULL,
               content_bytes INTEGER NOT NULL,
               workspace_id TEXT NOT NULL
             )`,
	`CREATE TABLE IF NOT EXISTS memory_records (
               memory_id TEXT PRIMARY KEY,
               record_revision INTEGER NOT NULL,
               kind TEXT NOT NULL,
               tenant_id TEXT NOT NULL,
               workspace_id TEXT NOT NULL,
               session_id TEXT,
               trust TEXT NOT NULL,
               sensitivity TEXT NOT NULL,
               created_at TEXT NOT NULL,
               effective_at TEXT NOT NULL,
               expires_at TEXT,
               tombstone INTEGER NOT NULL CHECK(tombstone IN (0, 1)),
               content_bytes INTEGER NOT NULL,
               record_json TEXT NOT NULL
             )`,
	`CREATE TABLE IF NOT EXISTS memory_supersedes (
               memory_id TEXT NOT NULL,
               superseded_id TEXT NOT NULL,
               PRIMARY KEY(memory_id, superseded_id),
               FOREIGN KEY(memory_id) REFERENCES memory_records(memory_id) ON DELETE CASCADE
             )`,
	`CREATE TABLE IF NOT EXISTS memory_apply_journal (
               idempotency_key TEXT PRIMARY KEY,
               proposal_id TEXT NOT NULL,
               request_id TEXT NOT NULL,
               content_digest TEXT NOT NULL,
               record_id TEXT NOT NULL,
               store_revision INTEGER NOT NULL,
               record_revision INTEGER NOT NULL,
               actor_id TEXT NOT NULL,
               applied_at TEXT NOT NULL
             )`,
}

const ftsDDL = `CREATE VIRTUAL TABLE IF NOT EXISTS memory_fts USING fts5(
               memory_id UNINDEXED,
               tenant_id UNINDEXED,
               workspace_id UNINDEXED,
               session_id UNINDEXED,
               content,
               tokenize='unicode61'
             )`

const insertMetaSQL = `INSERT INTO schema_meta(
                       component, schema_version, store_revision, record_count,
                       content_bytes, workspace_id
                     ) VALUES (?, ?, 0, 0, 0, ?)`

const selectMetaSQL = `SELECT schema_version, store_revision, workspace_id
             FROM schema_meta WHERE component = ?`

// selectMetadataSQL also reads the bounded-store counters so StoreMetadata
// carries them (Rust metadata() selects only the identity columns).
const selectMetadataSQL = `SELECT schema_version, store_revision, record_count, content_bytes, workspace_id
             FROM schema_meta WHERE component = ?`

// initialize runs the schema DDL and the schema_meta identity check in one
// transaction, inserting the meta row for a fresh database.
func (s *Store) initialize(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return persistenceErr(s.path, err)
	}
	defer tx.Rollback()
	for _, ddl := range schemaDDL {
		if _, err := tx.ExecContext(ctx, ddl); err != nil {
			return persistenceErr(s.path, err)
		}
	}
	if _, err := tx.ExecContext(ctx, ftsDDL); err != nil {
		return ftsUnavailableErr(err)
	}
	var version, revision int64
	var storedWorkspace string
	err = tx.QueryRowContext(ctx, selectMetaSQL, component).
		Scan(&version, &revision, &storedWorkspace)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		_, err = tx.ExecContext(ctx, insertMetaSQL, component, schemaVersion, s.workspaceID)
		if err != nil {
			return persistenceErr(s.path, err)
		}
	case err != nil:
		return persistenceErr(s.path, err)
	default:
		if err := checkMetaRow(version, storedWorkspace, s.workspaceID); err != nil {
			return err
		}
	}
	if err := tx.Commit(); err != nil {
		return persistenceErr(s.path, err)
	}
	return nil
}

// checkIdentity validates schema version and workspace on an existing
// database without modifying it.
func (s *Store) checkIdentity(ctx context.Context) error {
	var version, revision int64
	var storedWorkspace string
	err := s.db.QueryRowContext(ctx, selectMetaSQL, component).
		Scan(&version, &revision, &storedWorkspace)
	if err != nil {
		return persistenceErr(s.path, err)
	}
	return checkMetaRow(version, storedWorkspace, s.workspaceID)
}

func checkMetaRow(version int64, storedWorkspace, workspaceID string) error {
	if version != schemaVersion {
		return unsupportedSchemaErr(version)
	}
	if storedWorkspace != workspaceID {
		return databaseWorkspaceMismatchErr(workspaceID, storedWorkspace)
	}
	return nil
}
