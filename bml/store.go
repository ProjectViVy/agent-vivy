package bml

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"modernc.org/sqlite"
)

const (
	schemaVersion int64 = 1
	component           = "embedded_laputa"
	storeFileName       = "memory.sqlite3"
	busyTimeoutMS       = 5000
	// validationClockSkew is the future-timestamp tolerance on every write,
	// matching chrono::Duration::minutes(5) in typed_store.rs.
	validationClockSkew = 5 * time.Minute
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
	ErrCorruptRecord             StoreErrorCode = "corrupt_record"
	ErrImportConflict            StoreErrorCode = "import_conflict"
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

func corruptRecordErr() *StoreError {
	return &StoreError{
		Code:    ErrCorruptRecord,
		Message: "stored Memory row is corrupt",
	}
}

func importConflictErr(recordID string) *StoreError {
	return &StoreError{
		Code:    ErrImportConflict,
		Message: fmt.Sprintf("imported Memory record %s conflicts with existing content", recordID),
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

// StoredRecord is the canonical record plus the row revision owned by the
// store (StoredMemoryRecord in typed_store.rs).
type StoredRecord struct {
	Record   Record
	Revision int64
}

// targetRevisionGuard carries put_tombstone's extra CAS precondition: the
// tombstone is written only while the target record is still at the revision
// the caller observed.
type targetRevisionGuard struct {
	id       string
	revision int64
}

const (
	selectStoreRevisionSQL  = `SELECT store_revision FROM schema_meta WHERE component = ?`
	reserveStoreRevisionSQL = `UPDATE schema_meta SET store_revision = store_revision + 1
             WHERE component = ? AND store_revision = ?`
	selectRecordRevisionSQL = `SELECT record_revision FROM memory_records WHERE memory_id = ?`
	selectCountersSQL       = `SELECT record_count, content_bytes FROM schema_meta WHERE component = ?`
	selectContentBytesSQL   = `SELECT COALESCE((SELECT content_bytes FROM memory_records WHERE memory_id = ?), 0)`
	selectStoredSQL         = `SELECT record_revision, record_json FROM memory_records WHERE memory_id = ?`
	selectRecordJSONSQL     = `SELECT record_json FROM memory_records WHERE memory_id = ?`
	listStoredSQL           = `SELECT record_revision, record_json FROM memory_records
             ORDER BY memory_id LIMIT ?`
	selectSupersededTargetsSQL = `SELECT s.superseded_id
             FROM memory_supersedes s
             JOIN memory_records r ON r.memory_id = s.memory_id
             WHERE r.tombstone = 1`
	selectSuppressedByTombstoneSQL = `SELECT COUNT(*) FROM memory_supersedes s
             JOIN memory_records r ON r.memory_id = s.memory_id
             WHERE s.superseded_id = ? AND r.tombstone = 1`
	upsertRecordSQL = `INSERT INTO memory_records(
               memory_id, record_revision, kind, tenant_id, workspace_id, session_id,
               trust, sensitivity, created_at, effective_at, expires_at, tombstone,
               content_bytes, record_json
             ) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
             ON CONFLICT(memory_id) DO UPDATE SET
               record_revision=excluded.record_revision, kind=excluded.kind,
               tenant_id=excluded.tenant_id, workspace_id=excluded.workspace_id,
               session_id=excluded.session_id, trust=excluded.trust,
               sensitivity=excluded.sensitivity, created_at=excluded.created_at,
               effective_at=excluded.effective_at, expires_at=excluded.expires_at,
               tombstone=excluded.tombstone, content_bytes=excluded.content_bytes,
               record_json=excluded.record_json`
	insertRecordSQL = `INSERT INTO memory_records(
               memory_id, record_revision, kind, tenant_id, workspace_id, session_id,
               trust, sensitivity, created_at, effective_at, expires_at, tombstone,
               content_bytes, record_json
             ) VALUES (?, 1, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`
	deleteSupersedesSQL = `DELETE FROM memory_supersedes WHERE memory_id = ?`
	insertSupersedesSQL = `INSERT INTO memory_supersedes(memory_id, superseded_id) VALUES (?, ?)`
	deleteFTSSQL        = `DELETE FROM memory_fts WHERE memory_id = ?`
	insertFTSSQL        = `INSERT INTO memory_fts(memory_id, tenant_id, workspace_id, session_id, content)
             VALUES (?, ?, ?, ?, ?)`
	updateCountersSQL = `UPDATE schema_meta SET record_count = ?, content_bytes = ?
             WHERE component = ?`
	bumpImportMetaSQL = `UPDATE schema_meta
             SET store_revision = store_revision + ?, record_count = ?, content_bytes = ?
             WHERE component = ?`
)

// Get returns the stored record for id, or nil when absent. Tombstone
// records are returned like any other row; read-side visibility rules are a
// search/list concern.
func (s *Store) Get(ctx context.Context, id string) (*StoredRecord, error) {
	var revision int64
	var recordJSON string
	err := s.db.QueryRowContext(ctx, selectStoredSQL, id).Scan(&revision, &recordJSON)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, persistenceErr(s.path, err)
	}
	stored, err := decodeStored(revision, recordJSON)
	if err != nil {
		return nil, err
	}
	return &stored, nil
}

// List returns records in deterministic memory_id order, capped at limit and
// at the bounded-store capacity.
func (s *Store) List(ctx context.Context, limit uint32) ([]StoredRecord, error) {
	if int64(limit) > MaxMemoryRecords {
		limit = uint32(MaxMemoryRecords)
	}
	rows, err := s.db.QueryContext(ctx, listStoredSQL, limit)
	if err != nil {
		return nil, persistenceErr(s.path, err)
	}
	defer rows.Close()
	var out []StoredRecord
	for rows.Next() {
		var revision int64
		var recordJSON string
		if err := rows.Scan(&revision, &recordJSON); err != nil {
			return nil, persistenceErr(s.path, err)
		}
		stored, err := decodeStored(revision, recordJSON)
		if err != nil {
			return nil, err
		}
		out = append(out, stored)
	}
	if err := rows.Err(); err != nil {
		return nil, persistenceErr(s.path, err)
	}
	return out, nil
}

// SupersededTargetIDs returns the IDs targeted by any supersedes tombstone
// currently in the store. Read-side projections use this to exclude records
// deposed by a later tombstone even when the target row has no tombstone
// flag set.
func (s *Store) SupersededTargetIDs(ctx context.Context) (map[string]struct{}, error) {
	rows, err := s.db.QueryContext(ctx, selectSupersededTargetsSQL)
	if err != nil {
		return nil, persistenceErr(s.path, err)
	}
	defer rows.Close()
	targets := map[string]struct{}{}
	for rows.Next() {
		var target string
		if err := rows.Scan(&target); err != nil {
			return nil, persistenceErr(s.path, err)
		}
		targets[target] = struct{}{}
	}
	if err := rows.Err(); err != nil {
		return nil, persistenceErr(s.path, err)
	}
	return targets, nil
}

// Put inserts or replaces one canonical record under store and row CAS.
// expectedRecordRevision nil means the record must not exist yet.
func (s *Store) Put(ctx context.Context, record Record, expectedStoreRevision int64, expectedRecordRevision *int64) (StoredRecord, error) {
	return s.putInner(ctx, record, expectedStoreRevision, expectedRecordRevision, nil)
}

// PutTombstone atomically writes a tombstone only if the target record still
// has the revision observed by the caller.
func (s *Store) PutTombstone(ctx context.Context, record Record, expectedStoreRevision int64, targetID string, targetRevision int64) error {
	_, err := s.putInner(ctx, record, expectedStoreRevision, nil,
		&targetRevisionGuard{id: targetID, revision: targetRevision})
	return err
}

// putInner mirrors typed_store.rs put_inner minus the governed-apply seam:
// validate -> workspace check -> store CAS reserve -> optional target guard
// -> record revision check -> capacity -> upsert -> supersedes -> FTS ->
// counters, all in one transaction.
func (s *Store) putInner(
	ctx context.Context,
	record Record,
	expectedStoreRevision int64,
	expectedRecordRevision *int64,
	targetGuard *targetRevisionGuard,
) (StoredRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if verr := record.ValidateAt(time.Now().UTC(), validationClockSkew); verr != nil {
		return StoredRecord{}, invalidRecordErr(verr)
	}
	if record.ValidateWorkspace(s.workspaceID) != nil {
		return StoredRecord{}, workspaceMismatchErr(s.workspaceID, record.Scope.WorkspaceID)
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return StoredRecord{}, persistenceErr(s.path, err)
	}
	defer tx.Rollback()

	var actualStore int64
	if err := tx.QueryRowContext(ctx, selectStoreRevisionSQL, component).Scan(&actualStore); err != nil {
		return StoredRecord{}, persistenceErr(s.path, err)
	}
	if actualStore != expectedStoreRevision {
		return StoredRecord{}, storeRevisionConflictErr(expectedStoreRevision, actualStore)
	}
	if targetGuard != nil {
		actualTarget, err := s.optionalRevision(ctx, tx, targetGuard.id)
		if err != nil {
			return StoredRecord{}, err
		}
		if actualTarget == nil || *actualTarget != targetGuard.revision {
			expected := targetGuard.revision
			return StoredRecord{}, recordRevisionConflictErr(targetGuard.id, &expected, actualTarget)
		}
	}
	reserved, err := tx.ExecContext(ctx, reserveStoreRevisionSQL, component, expectedStoreRevision)
	if err != nil {
		if isSQLiteBusy(err) {
			_ = tx.Rollback()
			meta, merr := s.Metadata(ctx)
			if merr != nil {
				return StoredRecord{}, merr
			}
			return StoredRecord{}, storeRevisionConflictErr(expectedStoreRevision, meta.StoreRevision)
		}
		return StoredRecord{}, persistenceErr(s.path, err)
	}
	if affected, err := reserved.RowsAffected(); err != nil || affected != 1 {
		var actual int64
		if err := tx.QueryRowContext(ctx, selectStoreRevisionSQL, component).Scan(&actual); err != nil {
			return StoredRecord{}, persistenceErr(s.path, err)
		}
		return StoredRecord{}, storeRevisionConflictErr(expectedStoreRevision, actual)
	}

	actualRecord, err := s.optionalRevision(ctx, tx, record.ID)
	if err != nil {
		return StoredRecord{}, err
	}
	if !equalRevisions(actualRecord, expectedRecordRevision) {
		return StoredRecord{}, recordRevisionConflictErr(record.ID, expectedRecordRevision, actualRecord)
	}

	var currentCount, currentBytes int64
	if err := tx.QueryRowContext(ctx, selectCountersSQL, component).Scan(&currentCount, &currentBytes); err != nil {
		return StoredRecord{}, persistenceErr(s.path, err)
	}
	var replacedBytes int64
	if err := tx.QueryRowContext(ctx, selectContentBytesSQL, record.ID).Scan(&replacedBytes); err != nil {
		return StoredRecord{}, persistenceErr(s.path, err)
	}
	nextCount := currentCount
	if actualRecord == nil {
		nextCount++
	}
	nextBytes := currentBytes - replacedBytes + int64(len(record.Content))
	if nextCount > MaxMemoryRecords || nextBytes > MaxMemoryContentBytes {
		return StoredRecord{}, capacityExceededErr(nextCount, nextBytes)
	}

	nextRecordRevision := int64(1)
	if actualRecord != nil {
		nextRecordRevision = *actualRecord + 1
	}
	recordJSON, err := json.Marshal(record)
	if err != nil {
		return StoredRecord{}, corruptRecordErr()
	}
	if _, err := tx.ExecContext(ctx, upsertRecordSQL,
		record.ID, nextRecordRevision, string(record.Kind),
		record.Scope.TenantID, record.Scope.WorkspaceID, record.Scope.SessionID,
		string(record.Trust), string(record.Sensitivity),
		rfc3339UTC(record.CreatedAt), rfc3339UTC(record.EffectiveAt),
		optionalRFC3339(record.ExpiresAt), boolInt(record.Tombstone != nil),
		int64(len(record.Content)), string(recordJSON),
	); err != nil {
		return StoredRecord{}, persistenceErr(s.path, err)
	}

	if _, err := tx.ExecContext(ctx, deleteSupersedesSQL, record.ID); err != nil {
		return StoredRecord{}, persistenceErr(s.path, err)
	}
	for _, supersededID := range record.Supersedes {
		if _, err := tx.ExecContext(ctx, insertSupersedesSQL, record.ID, supersededID); err != nil {
			return StoredRecord{}, persistenceErr(s.path, err)
		}
	}
	if _, err := tx.ExecContext(ctx, deleteFTSSQL, record.ID); err != nil {
		return StoredRecord{}, persistenceErr(s.path, err)
	}
	var suppressedByTombstone int64
	if err := tx.QueryRowContext(ctx, selectSuppressedByTombstoneSQL, record.ID).Scan(&suppressedByTombstone); err != nil {
		return StoredRecord{}, persistenceErr(s.path, err)
	}
	if record.Tombstone == nil && suppressedByTombstone == 0 {
		if _, err := tx.ExecContext(ctx, insertFTSSQL,
			record.ID, record.Scope.TenantID, record.Scope.WorkspaceID,
			record.Scope.SessionID, record.Content,
		); err != nil {
			return StoredRecord{}, persistenceErr(s.path, err)
		}
	}
	if record.Tombstone != nil {
		for _, supersededID := range record.Supersedes {
			if _, err := tx.ExecContext(ctx, deleteFTSSQL, supersededID); err != nil {
				return StoredRecord{}, persistenceErr(s.path, err)
			}
		}
	}
	if _, err := tx.ExecContext(ctx, updateCountersSQL, nextCount, nextBytes, component); err != nil {
		return StoredRecord{}, persistenceErr(s.path, err)
	}
	if err := tx.Commit(); err != nil {
		return StoredRecord{}, persistenceErr(s.path, err)
	}
	return StoredRecord{Record: record, Revision: nextRecordRevision}, nil
}

// ImportRecords inserts a deterministic record set in one transaction.
// Records already present with identical canonical JSON are idempotent
// replays; any conflicting ID or validation failure aborts the complete
// import without changing the store. Returns the number of newly inserted
// records.
func (s *Store) ImportRecords(ctx context.Context, records []Record) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now().UTC()
	for i := range records {
		if verr := records[i].ValidateAt(now, validationClockSkew); verr != nil {
			return 0, invalidRecordErr(verr)
		}
		if records[i].ValidateWorkspace(s.workspaceID) != nil {
			return 0, workspaceMismatchErr(s.workspaceID, records[i].Scope.WorkspaceID)
		}
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, persistenceErr(s.path, err)
	}
	defer tx.Rollback()

	type insert struct {
		record    *Record
		canonical string
	}
	var inserts []insert
	for i := range records {
		record := &records[i]
		canonicalJSON, err := json.Marshal(record)
		if err != nil {
			return 0, corruptRecordErr()
		}
		canonical := string(canonicalJSON)
		var existing string
		err = tx.QueryRowContext(ctx, selectRecordJSONSQL, record.ID).Scan(&existing)
		switch {
		case errors.Is(err, sql.ErrNoRows):
			inserts = append(inserts, insert{record: record, canonical: canonical})
		case err != nil:
			return 0, persistenceErr(s.path, err)
		case existing == canonical:
			continue
		default:
			return 0, importConflictErr(record.ID)
		}
	}

	var currentCount, currentBytes int64
	if err := tx.QueryRowContext(ctx, selectCountersSQL, component).Scan(&currentCount, &currentBytes); err != nil {
		return 0, persistenceErr(s.path, err)
	}
	nextCount := currentCount + int64(len(inserts))
	nextBytes := currentBytes
	for _, ins := range inserts {
		nextBytes += int64(len(ins.record.Content))
	}
	if nextCount > MaxMemoryRecords || nextBytes > MaxMemoryContentBytes {
		return 0, capacityExceededErr(nextCount, nextBytes)
	}

	for _, ins := range inserts {
		record := ins.record
		if _, err := tx.ExecContext(ctx, insertRecordSQL,
			record.ID, string(record.Kind),
			record.Scope.TenantID, record.Scope.WorkspaceID, record.Scope.SessionID,
			string(record.Trust), string(record.Sensitivity),
			rfc3339UTC(record.CreatedAt), rfc3339UTC(record.EffectiveAt),
			optionalRFC3339(record.ExpiresAt), boolInt(record.Tombstone != nil),
			int64(len(record.Content)), ins.canonical,
		); err != nil {
			return 0, persistenceErr(s.path, err)
		}
		for _, supersededID := range record.Supersedes {
			if _, err := tx.ExecContext(ctx, insertSupersedesSQL, record.ID, supersededID); err != nil {
				return 0, persistenceErr(s.path, err)
			}
		}
		if record.Tombstone == nil {
			if _, err := tx.ExecContext(ctx, insertFTSSQL,
				record.ID, record.Scope.TenantID, record.Scope.WorkspaceID,
				record.Scope.SessionID, record.Content,
			); err != nil {
				return 0, persistenceErr(s.path, err)
			}
		}
	}
	if len(inserts) > 0 {
		if _, err := tx.ExecContext(ctx, bumpImportMetaSQL, int64(len(inserts)), nextCount, nextBytes, component); err != nil {
			return 0, persistenceErr(s.path, err)
		}
	}
	if err := tx.Commit(); err != nil {
		return 0, persistenceErr(s.path, err)
	}
	return len(inserts), nil
}

// optionalRevision returns the record's revision, or nil when absent.
func (s *Store) optionalRevision(ctx context.Context, tx *sql.Tx, memoryID string) (*int64, error) {
	var revision sql.NullInt64
	err := tx.QueryRowContext(ctx, selectRecordRevisionSQL, memoryID).Scan(&revision)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, persistenceErr(s.path, err)
	}
	if !revision.Valid {
		return nil, nil
	}
	return &revision.Int64, nil
}

func equalRevisions(actual, expected *int64) bool {
	if actual == nil || expected == nil {
		return actual == nil && expected == nil
	}
	return *actual == *expected
}

func decodeStored(revision int64, recordJSON string) (StoredRecord, error) {
	var record Record
	if err := json.Unmarshal([]byte(recordJSON), &record); err != nil {
		return StoredRecord{}, corruptRecordErr()
	}
	return StoredRecord{Record: record, Revision: revision}, nil
}

func boolInt(v bool) int64 {
	if v {
		return 1
	}
	return 0
}

func optionalRFC3339(t *time.Time) any {
	if t == nil {
		return nil
	}
	return rfc3339UTC(*t)
}

// rfc3339UTC formats like chrono's DateTime<Utc>::to_rfc3339(): UTC "+00:00"
// offset with sub-second precision in AutoSi 0/3/6/9-digit groups.
func rfc3339UTC(t time.Time) string {
	t = t.UTC()
	ns := t.Nanosecond()
	base := t.Format("2006-01-02T15:04:05")
	switch {
	case ns == 0:
	case ns%1_000_000 == 0:
		base += fmt.Sprintf(".%03d", ns/1_000_000)
	case ns%1_000 == 0:
		base += fmt.Sprintf(".%06d", ns/1_000)
	default:
		base += fmt.Sprintf(".%09d", ns)
	}
	return base + "+00:00"
}

// isSQLiteBusy mirrors is_sqlite_busy in typed_store.rs: SQLITE_BUSY (5),
// SQLITE_LOCKED (6), SQLITE_BUSY_RECOVERY (261), SQLITE_BUSY_SNAPSHOT (517).
func isSQLiteBusy(err error) bool {
	var se *sqlite.Error
	if !errors.As(err, &se) {
		return false
	}
	switch se.Code() {
	case 5, 6, 261, 517:
		return true
	}
	return false
}
