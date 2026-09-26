// home.go ports Diva's bml/memory_home.rs: the machine-wide MemoryHome
// facade — lazy store handles, revisioned long-term CRUD, MEMRULES, session
// GC, and the startup L1 index projection. MemoryProvider runtime assembly
// (prefetch/sync_turn/checkpoint/session_end/system_prompt/recall_outcome/
// actmem_*) is intentionally out of scope.
package bml

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"unicode/utf8"
)

// machineMemoryScope is the workspace identity every machine-home record is
// bound to (MACHINE_MEMORY_SCOPE in memory_home.rs).
const machineMemoryScope = "machine-memory-home"

const memRulesFileName = "MEMRULES.MD"

// Stable machine-matchable HomeError codes (MemoryHomeError::code()).
const (
	HomeCodeBmlUnavailable   = "bml_unavailable"
	HomeCodeRevisionConflict = "memory_revision_conflict"
	HomeCodeKindForbidden    = "memory_kind_forbidden"
	HomeCodeNotFound         = "memory_not_found"
	HomeCodeInvalidRequest   = "memory_invalid_request"
	HomeCodeIO               = "memory_io_error"
)

// HomeError is a stable MemoryHome facade failure. The message never embeds
// Memory record content.
type HomeError struct {
	code    string
	message string
	err     error

	// Path is set for memory_io_error.
	Path string
	// ID is set for memory_not_found.
	ID string
	// Expected/Actual carry the CAS revisions for memory_revision_conflict.
	Expected int64
	Actual   *int64
}

func (e *HomeError) Error() string { return e.message }
func (e *HomeError) Unwrap() error { return e.err }

// Code returns the stable snake_case failure code.
func (e *HomeError) Code() string { return e.code }

func homeBmlUnavailable(err error) *HomeError {
	return &HomeError{
		code:    HomeCodeBmlUnavailable,
		message: "BML is unavailable: " + err.Error(),
		err:     err,
	}
}

func homeNotFound(id string) *HomeError {
	return &HomeError{
		code:    HomeCodeNotFound,
		message: "memory record not found: " + id,
		ID:      id,
	}
}

func homeRevisionConflict(expected int64, actual *int64) *HomeError {
	actualStr := "None"
	if actual != nil {
		actualStr = fmt.Sprintf("Some(%d)", *actual)
	}
	return &HomeError{
		code:     HomeCodeRevisionConflict,
		message:  fmt.Sprintf("memory revision conflict: expected %d, actual %s", expected, actualStr),
		Expected: expected,
		Actual:   actual,
	}
}

func homeKindForbidden() *HomeError {
	return &HomeError{
		code:    HomeCodeKindForbidden,
		message: "memory kind is forbidden in production BML",
	}
}

func homeInvalid(msg string) *HomeError {
	return &HomeError{
		code:    HomeCodeInvalidRequest,
		message: "invalid memory request: " + msg,
	}
}

// mapRevisionError ports map_revision_error: a record-level CAS failure
// becomes memory_revision_conflict; every other store failure is
// bml_unavailable.
func mapRevisionError(err error) error {
	var se *StoreError
	if errors.As(err, &se) && se.Code == ErrRecordRevisionConflict {
		var expected int64
		if se.ExpectedRevision != nil {
			expected = *se.ExpectedRevision
		}
		return homeRevisionConflict(expected, se.ActualRevision)
	}
	return homeBmlUnavailable(err)
}

// MemRulesSource identifies whether MEMRULES content came from the built-in
// rulebook or the on-disk file. Wire spellings are serde snake_case.
type MemRulesSource string

const (
	MemRulesSourceDefault MemRulesSource = "default"
	MemRulesSourceFile    MemRulesSource = "file"
)

// MemRulesDocument is the current MEMRULES.MD content plus its origin.
type MemRulesDocument struct {
	Content string         `json:"content"`
	Source  MemRulesSource `json:"source"`
}

// DefaultMemRulesText is the built-in fallback rulebook, ported verbatim
// from DEFAULT_MEM_RULES_TEXT in Diva's cognitive/memrules.rs.
const DefaultMemRulesText = `---
version: 1
updated: 2026-08-07T00:00:00Z
---

# Memory Rules

## R1 — Evidence primacy
Typed memory records' evidence_refs chains are the primary source of
truth. Memory writes without evidence stay advisory.

## R2 — Claim distinction
Confirmed fact, observation, inference, and hypothesis are distinct
categories and must not be conflated.

## R3 — Contradiction handling
New contradictory evidence does not silently overwrite prior
understanding; conflicts surface for review.

## R4 — User authority
User-confirmed information outranks agent inference. Memory writes apply
directly to BML with revision checks; evidence remains advisory.

## R5 — Scope constraint
Scope, time, confidence, provenance, and visibility constrain how a
memory may be used.

## R6 — WORLD entry gate
Entry into WORLD requires action relevance and a bounded, reviewable
claim.

## R7 — No wholesale injection
WORLD is never copied wholesale into an agent context; only bounded,
scope-matched projections may be used.
`

// Home is the machine-wide BML authority facade (MemoryHome in
// memory_home.rs). The store handle is lazy: reads against a missing
// database project empty results instead of creating it.
type Home struct {
	configDir    string
	memoryDir    string
	database     string
	memrulesPath string
	l1IndexLines int

	mu    sync.Mutex
	store *Store

	indexMu         sync.RWMutex
	startupMarkdown string
	startupRevision atomic.Uint64
}

// NewHome opens a machine MemoryHome rooted at configDir with the default
// L1 index budget.
func NewHome(configDir string) *Home {
	return NewHomeWithL1Budget(configDir, DefaultL1IndexLines)
}

// NewHomeWithL1Budget opens a machine MemoryHome rooted at configDir with a
// caller-supplied L1 index line budget.
func NewHomeWithL1Budget(configDir string, l1IndexLines int) *Home {
	memoryDir := filepath.Join(configDir, "memory")
	return &Home{
		configDir:       configDir,
		memoryDir:       memoryDir,
		database:        filepath.Join(memoryDir, storeFileName),
		memrulesPath:    filepath.Join(memoryDir, memRulesFileName),
		l1IndexLines:    l1IndexLines,
		startupMarkdown: memrulesPointer(),
	}
}

// ConfigDir returns the configuration root the home was opened with.
func (h *Home) ConfigDir() string { return h.configDir }

// MemoryDir returns {config}/memory, the directory holding the database and
// MEMRULES.MD.
func (h *Home) MemoryDir() string { return h.memoryDir }

// DatabasePath returns {config}/memory/memory.sqlite3.
func (h *Home) DatabasePath() string { return h.database }

// MemRulesPath returns {config}/memory/MEMRULES.MD.
func (h *Home) MemRulesPath() string { return h.memrulesPath }

// StartupMarkdown is the current L1 startup projection: the MEMRULES
// pointer plus the rendered index block when the authority is non-empty.
func (h *Home) StartupMarkdown() string {
	h.indexMu.RLock()
	defer h.indexMu.RUnlock()
	return h.startupMarkdown
}

// StartupRevision increments every time the startup projection changes.
func (h *Home) StartupRevision() uint64 {
	return h.startupRevision.Load()
}

// Close releases the cached store handle, if any.
func (h *Home) Close() error {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.store == nil {
		return nil
	}
	err := h.store.Close()
	h.store = nil
	return err
}

// Warmup warms the read-side index only when the machine-home database
// exists; a missing database stays absent and projects an empty index.
func (h *Home) Warmup(ctx context.Context) error {
	return h.refreshStartupIndex(ctx)
}

// existingStore returns the cached store, opening an existing database
// read-write without creating it. A missing database yields nil.
func (h *Home) existingStore(ctx context.Context) (*Store, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.store != nil {
		return h.store, nil
	}
	if !isFile(h.database) {
		return nil, nil
	}
	store, err := openExistingMutable(ctx, h.database, machineMemoryScope)
	if err != nil {
		return nil, homeBmlUnavailable(err)
	}
	h.store = store
	return store, nil
}

// writableStore returns the cached store or creates/opens the database.
func (h *Home) writableStore(ctx context.Context) (*Store, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.store != nil {
		return h.store, nil
	}
	store, err := openPath(ctx, h.database, machineMemoryScope)
	if err != nil {
		return nil, homeBmlUnavailable(err)
	}
	h.store = store
	return store, nil
}

// openExistingMutable opens the database at path read-write with no schema
// initialization, failing when the file is absent or bound to another
// workspace (TypedMemoryStore::open_existing_database).
func openExistingMutable(ctx context.Context, path, workspaceID string) (*Store, error) {
	if !isFile(path) {
		return nil, invalidBackupErr()
	}
	// mode=rw keeps the open honest: a file deleted between the isFile
	// check and connect errors instead of silently recreating an empty DB.
	db, err := sql.Open("sqlite", fmt.Sprintf("file:%s?mode=rw&_pragma=journal_mode(WAL)&_pragma=foreign_keys(1)&_pragma=busy_timeout(%d)", path, busyTimeoutMS))
	if err != nil {
		return nil, ioErr(path, err)
	}
	db.SetMaxOpenConns(4)
	s := &Store{db: db, path: path, workspaceID: workspaceID}
	if err := s.checkIdentity(ctx); err != nil {
		_ = db.Close()
		return nil, err
	}
	return s, nil
}

// ListRecords returns the visible long-term records, capped at limit. A
// missing store yields an empty list.
func (h *Home) ListRecords(ctx context.Context, limit uint32) ([]StoredRecord, error) {
	store, err := h.existingStore(ctx)
	if err != nil {
		return nil, err
	}
	if store == nil {
		return []StoredRecord{}, nil
	}
	superseded, err := store.SupersededTargetIDs(ctx)
	if err != nil {
		return nil, homeBmlUnavailable(err)
	}
	stored, err := store.List(ctx, limit)
	if err != nil {
		return nil, homeBmlUnavailable(err)
	}
	out := make([]StoredRecord, 0, len(stored))
	for _, s := range stored {
		if visibleLongTerm(&s.Record, superseded) {
			out = append(out, entryFromStored(s))
		}
	}
	return out, nil
}

// GetRecord returns the visible long-term record for id, or nil. A missing
// store yields nil.
func (h *Home) GetRecord(ctx context.Context, id string) (*StoredRecord, error) {
	store, err := h.existingStore(ctx)
	if err != nil {
		return nil, err
	}
	if store == nil {
		return nil, nil
	}
	superseded, err := store.SupersededTargetIDs(ctx)
	if err != nil {
		return nil, homeBmlUnavailable(err)
	}
	stored, err := store.Get(ctx, id)
	if err != nil {
		return nil, homeBmlUnavailable(err)
	}
	if stored == nil || !visibleLongTerm(&stored.Record, superseded) {
		return nil, nil
	}
	entry := entryFromStored(*stored)
	return &entry, nil
}

// AddLongTerm appends a long-term record (add_long_term in memory_home.rs).
func (h *Home) AddLongTerm(ctx context.Context, content string, evidence []EvidenceRef) (StoredRecord, error) {
	return h.AddRecord(ctx, KindLongTerm, content, evidence)
}

// AddRecord writes a new record. Only KindLongTerm is permitted in the
// production BML; any other kind fails with memory_kind_forbidden.
func (h *Home) AddRecord(ctx context.Context, kind Kind, content string, evidence []EvidenceRef) (StoredRecord, error) {
	if kind != KindLongTerm {
		return StoredRecord{}, homeKindForbidden()
	}
	content = strings.TrimSpace(content)
	if content == "" {
		return StoredRecord{}, homeInvalid("content is empty")
	}
	store, err := h.writableStore(ctx)
	if err != nil {
		return StoredRecord{}, err
	}
	now := time.Now().UTC()
	digest := MemoryContentDigest([]byte(content))
	id := fmt.Sprintf("memory-%d-%s", now.UnixMicro(), digest.Value[:12])
	record := longTermRecord(id, content, evidence, now)
	metadata, err := store.Metadata(ctx)
	if err != nil {
		return StoredRecord{}, homeBmlUnavailable(err)
	}
	stored, err := store.Put(ctx, record, metadata.StoreRevision, nil)
	if err != nil {
		return StoredRecord{}, homeBmlUnavailable(err)
	}
	if err := h.refreshStartupIndex(ctx); err != nil {
		return StoredRecord{}, err
	}
	return entryFromStored(stored), nil
}

// UpdateRecord rewrites content and evidence under record-revision CAS,
// refreshing provenance to the user_input/memory_update origin.
func (h *Home) UpdateRecord(ctx context.Context, id, content string, baseRevision int64, evidence []EvidenceRef) (StoredRecord, error) {
	content = strings.TrimSpace(content)
	if content == "" {
		return StoredRecord{}, homeInvalid("content is empty")
	}
	store, err := h.existingStore(ctx)
	if err != nil {
		return StoredRecord{}, err
	}
	if store == nil {
		return StoredRecord{}, homeNotFound(id)
	}
	current, err := store.Get(ctx, id)
	if err != nil {
		return StoredRecord{}, homeBmlUnavailable(err)
	}
	if current == nil || current.Record.Kind != KindLongTerm || current.Record.Tombstone != nil {
		return StoredRecord{}, homeNotFound(id)
	}
	if current.Revision != baseRevision {
		actual := current.Revision
		return StoredRecord{}, homeRevisionConflict(baseRevision, &actual)
	}
	now := time.Now().UTC()
	record := current.Record
	record.Content = content
	record.EvidenceRefs = evidence
	record.EffectiveAt = now
	record.Provenance.Source = ProvenanceSourceUserInput
	record.Provenance.SourceID = "memory_update"
	record.Provenance.ContentDigest = MemoryContentDigest([]byte(record.Content))
	record.Provenance.CapturedAt = now
	record.Provenance.Correlation = correlation("memory_update", id)
	metadata, err := store.Metadata(ctx)
	if err != nil {
		return StoredRecord{}, homeBmlUnavailable(err)
	}
	stored, err := store.Put(ctx, record, metadata.StoreRevision, &baseRevision)
	if err != nil {
		return StoredRecord{}, mapRevisionError(err)
	}
	if err := h.refreshStartupIndex(ctx); err != nil {
		return StoredRecord{}, err
	}
	return entryFromStored(stored), nil
}

// RemoveRecord writes a tombstone over id under record-revision CAS and
// returns the deposed record as last observed.
func (h *Home) RemoveRecord(ctx context.Context, id, reason string, baseRevision int64) (StoredRecord, error) {
	reason = strings.TrimSpace(reason)
	if reason == "" {
		return StoredRecord{}, homeInvalid("reason is empty")
	}
	store, err := h.existingStore(ctx)
	if err != nil {
		return StoredRecord{}, err
	}
	if store == nil {
		return StoredRecord{}, homeNotFound(id)
	}
	current, err := store.Get(ctx, id)
	if err != nil {
		return StoredRecord{}, homeBmlUnavailable(err)
	}
	if current == nil || current.Record.Kind != KindLongTerm || current.Record.Tombstone != nil {
		return StoredRecord{}, homeNotFound(id)
	}
	if current.Revision != baseRevision {
		actual := current.Revision
		return StoredRecord{}, homeRevisionConflict(baseRevision, &actual)
	}
	now := time.Now().UTC()
	tombstone := Record{
		ID:      fmt.Sprintf("memory-tombstone-%d", now.UnixMicro()),
		Kind:    KindLongTerm,
		Content: "",
		Provenance: Provenance{
			Source:        ProvenanceSourceUserInput,
			SourceID:      "memory_remove",
			ContentDigest: MemoryContentDigest(nil),
			CapturedAt:    now,
			Correlation:   correlation("memory_remove", id),
		},
		EvidenceRefs:  []EvidenceRef{},
		ConfidenceBPS: MaxConfidenceBPS,
		Sensitivity:   SensitivityInternal,
		Trust:         TrustUserAsserted,
		Scope:         machineScope(nil),
		CreatedAt:     now,
		EffectiveAt:   now,
		Supersedes:    []string{id},
		Tombstone: &Tombstone{
			TargetRecordID: id,
			ReasonDigest:   MemoryContentDigest([]byte(reason)),
			ActorID:        "user",
			CreatedAt:      now,
		},
	}
	metadata, err := store.Metadata(ctx)
	if err != nil {
		return StoredRecord{}, homeBmlUnavailable(err)
	}
	if err := store.PutTombstone(ctx, tombstone, metadata.StoreRevision, id, baseRevision); err != nil {
		return StoredRecord{}, mapRevisionError(err)
	}
	if err := h.refreshStartupIndex(ctx); err != nil {
		return StoredRecord{}, err
	}
	return entryFromStored(*current), nil
}

// ReadMemRules returns the MEMRULES.MD content, falling back to the
// built-in rulebook when the file does not exist.
func (h *Home) ReadMemRules() (MemRulesDocument, error) {
	raw, err := os.ReadFile(h.memrulesPath)
	switch {
	case err == nil:
		if !utf8.Valid(raw) {
			// Rust read_to_string surfaces invalid UTF-8 as an io error,
			// which the facade maps to memory_invalid_request.
			return MemRulesDocument{}, homeInvalid("MEMRULES content is not valid UTF-8")
		}
		return MemRulesDocument{Content: string(raw), Source: MemRulesSourceFile}, nil
	case os.IsNotExist(err):
		return MemRulesDocument{Content: DefaultMemRulesText, Source: MemRulesSourceDefault}, nil
	default:
		return MemRulesDocument{}, homeInvalid(err.Error())
	}
}

// WriteMemRules atomically replaces MEMRULES.MD with normalized content.
func (h *Home) WriteMemRules(content string) (MemRulesDocument, error) {
	content = strings.ReplaceAll(content, "\r\n", "\n")
	if strings.TrimSpace(content) == "" {
		return MemRulesDocument{}, homeInvalid("MEMRULES content is empty")
	}
	// Rust atomic_write creates the parent directory; the shared helper
	// does not, so create it here to keep write_memrules self-contained.
	if err := os.MkdirAll(h.memoryDir, 0o755); err != nil {
		return MemRulesDocument{}, homeInvalid(err.Error())
	}
	if err := atomicWrite(h.memrulesPath, []byte(content)); err != nil {
		return MemRulesDocument{}, homeInvalid(err.Error())
	}
	return MemRulesDocument{Content: content, Source: MemRulesSourceFile}, nil
}

// ClearSessionCheckpoint physically deletes every record scoped to
// sessionID; a missing store reports zero removals.
func (h *Home) ClearSessionCheckpoint(ctx context.Context, sessionID string) (uint64, error) {
	store, err := h.existingStore(ctx)
	if err != nil {
		return 0, err
	}
	if store == nil {
		return 0, nil
	}
	n, err := store.GCSessionScoped(ctx, sessionID)
	if err != nil {
		return 0, homeBmlUnavailable(err)
	}
	return n, nil
}

// RunStartupGC deletes every session-scoped record whose session_id is not
// in activeSessionIDs. An empty list is a no-op (upstream parity:
// memory_home.rs early-returns Ok(0) before touching the store). A missing
// store reports zero removals.
func (h *Home) RunStartupGC(ctx context.Context, activeSessionIDs []string) (uint64, error) {
	if len(activeSessionIDs) == 0 {
		return 0, nil
	}
	store, err := h.existingStore(ctx)
	if err != nil {
		return 0, err
	}
	if store == nil {
		return 0, nil
	}
	n, err := store.GCStaleSessionScoped(ctx, activeSessionIDs)
	if err != nil {
		return 0, homeBmlUnavailable(err)
	}
	return n, nil
}

// refreshStartupIndex re-renders the L1 startup projection from the visible
// long-term authority and bumps the revision only when the rendered
// markdown changed (refresh_startup_index in memory_home.rs).
func (h *Home) refreshStartupIndex(ctx context.Context) error {
	entries, err := h.ListRecords(ctx, uint32(h.l1IndexLines))
	if err != nil {
		return err
	}
	pairs := make([]L1IndexEntry, 0, len(entries))
	for _, entry := range entries {
		pairs = append(pairs, L1IndexEntry{ID: entry.Record.ID, Content: entry.Record.Content})
	}
	index := RenderL1IndexBlock(pairs, h.l1IndexLines)
	rendered := memrulesPointer()
	if index != "" {
		rendered += "\n\n" + strings.TrimSpace(index)
	}
	h.indexMu.Lock()
	defer h.indexMu.Unlock()
	if h.startupMarkdown != rendered {
		h.startupMarkdown = rendered
		h.startupRevision.Add(1)
	}
	return nil
}

// longTermRecord builds the canonical record for a user memory add
// (long_term_record in memory_home.rs).
func longTermRecord(id, content string, evidence []EvidenceRef, now time.Time) Record {
	return Record{
		ID:      id,
		Kind:    KindLongTerm,
		Content: content,
		Provenance: Provenance{
			Source:        ProvenanceSourceUserInput,
			SourceID:      "memory_add",
			ContentDigest: MemoryContentDigest([]byte(content)),
			CapturedAt:    now,
			Correlation:   correlation("memory_add", id),
		},
		EvidenceRefs:  evidence,
		ConfidenceBPS: MaxConfidenceBPS,
		Sensitivity:   SensitivityInternal,
		Trust:         TrustUserAsserted,
		Scope:         machineScope(nil),
		CreatedAt:     now,
		EffectiveAt:   now,
		Supersedes:    []string{},
	}
}

// machineScope is the tenant-local, machine-memory-home scope every facade
// record carries (machine_scope in memory_home.rs).
func machineScope(sessionID *string) Scope {
	return Scope{
		TenantID:    "local",
		WorkspaceID: machineMemoryScope,
		SessionID:   sessionID,
	}
}

// correlation derives the audit correlation for a facade write
// (correlation in memory_home.rs).
func correlation(operation, key string) AuditCorrelation {
	return AuditCorrelation{
		RequestID: operation + "-" + key,
		TurnID:    operation,
		SessionID: key,
	}
}

// visibleLongTerm is the facade read filter: long-term, not tombstoned,
// machine-scoped, and not deposed by a supersedes tombstone
// (visible_long_term in memory_home.rs).
func visibleLongTerm(record *Record, superseded map[string]struct{}) bool {
	if record.Kind != KindLongTerm || record.Tombstone != nil || record.Scope.SessionID != nil {
		return false
	}
	_, found := superseded[record.ID]
	return !found
}

// entryFromStored is the facade's record projection. The Rust port targets
// MemoryEntry; on this surface it collapses to the stored record itself
// (entry_from_stored in memory_home.rs).
func entryFromStored(stored StoredRecord) StoredRecord {
	return stored
}

// memrulesPointer is the injection-safe pointer rendered when the startup
// projection has no index (memrules_pointer in memory_home.rs).
func memrulesPointer() string {
	return "## Memory Policy Pointer\n\nMemory writes must consult the machine-wide MEMRULES handbook. Full rules are injected only before a Memory or ACTMEM write tool executes."
}
