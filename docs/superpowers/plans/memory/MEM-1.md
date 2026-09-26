# MEM-1 BML library port Implementation Plan

> **For agentic workers:** Use superpowers:executing-plans for native execution, or superpowers:subagent-driven-development when that method is selected. Steps use checkbox syntax.

**Goal:** Port Diva's BML (Basic Memory Layer) to a standalone Go library living inside this repo, extractable later.
**Architecture:** New top-level Go module `bml/` (module `github.com/ProjectViVy/agent-vivy/bml`), package `bml`, porting `agent-diva-laputa`'s typed SQLite + FTS5 store, record model, MemoryHome facade, and offline migration adapters. No Vivy runtime/Port wiring in this Story.
**Tech Stack:** Go 1.26.4, `database/sql` + `modernc.org/sqlite` v1.56.0 (same version as root `go.mod`), stdlib only otherwise.
**Spec:** [design](../../specs/2026-09-26-memory-providers-design.md) REQ-MEM-1,6,7,13; issue #33 G1. State/dependencies: [index](index.md).
**Source of truth for porting:** `~/reference/agent-diva` pinned at `0fd005a1`. Port targets:
`agent-diva-core/src/memory/record.rs` (record model + validation),
`agent-diva-core/src/governance/types.rs` (ContentDigest/AuditCorrelation/DigestAlgorithm),
`agent-diva-core/src/evolution/types.rs` (EvidenceRef/EvidenceSource),
`agent-diva-laputa/src/typed_store.rs` (store),
`agent-diva-laputa/src/bml/memory_home.rs` (facade),
`agent-diva-laputa/src/memory_records.rs` (migration adapters),
`agent-diva-laputa/src/atomic.rs` (atomic file write).

## Global Constraints

- Standalone module: own `go.mod` at `bml/`; importable without the root module.
  Do not import `agent-vivy` root packages.
- No async — Go is synchronous; drop `tokio`/`.await`. All store methods take
  `context.Context` and are safe for concurrent use (Rust serializes via
  `SqlitePool`/mutexes; replicate with `*sql.DB` + a `sync.Mutex` for
  read-modify-write sequences).
- Port the schema byte-faithful from `typed_store.rs:494-560`
  (`schema_meta`, `memory_records`, `memory_supersedes`,
  `memory_apply_journal`, `memory_fts` with `tokenize='unicode61'`).
- **Drop the governed seam:** `put_governed` / `rollback_governed` /
  `GovernedMemoryApply` are dead upstream (see `bml/mod.rs` note) — do not port.
  Keep the `memory_apply_journal` table in the schema for byte-compat.
- **Defer ACTMEM**: `actmem.rs` (working-memory markdown store) and the
  `MemoryHome.actmem()` accessor are out of this Story's scope.
- **Defer Diva provider surface:** `MemoryProvider` trait methods
  (prefetch/sync_turn/checkpoint/session_end/system_prompt/recall_outcome/
  actmem_*) are runtime assembly, not BML storage — not ported.
- JSON field names must match serde `snake_case` exactly (the DB stores
  `record_json`; cross-language compatibility matters).
- Error types are typed Go errors exposing the same codes:
  `MemoryHomeError.Code()`: `bml_unavailable`, `memory_revision_conflict`,
  `memory_kind_forbidden`, `memory_not_found`, `memory_invalid_request`,
  `memory_io_error`.
- No `sqlx` equivalent exists — use `database/sql` with `modernc.org/sqlite`
  (driver name `sqlite`). Open with `_pragma=busy_timeout(5000)` and
  `_pragma=journal_mode(WAL)` equivalents modernc supports; verify FTS5 at open
  by executing the `CREATE VIRTUAL TABLE ... USING fts5` statement.
- Errors must never embed memory `Content` bytes in messages.
- `gofmt` clean; `go vet ./...` clean; `go test ./...` green in `bml/`.

## Review Focus

- FTS5 query escaping: user text must not break the MATCH expression (see
  `search`'s quoting in `typed_store.rs`).
- Revision conflicts: `store_revision` and per-record `record_revision` checks
  must reject concurrent writers, matching Rust semantics.
- Tombstones must never surface content via list/get/search.
- Workspace binding: opening a store for a different `workspace_id` must fail
  with `database_workspace_mismatch`.
- Capacity limits: `MAX_MEMORY_RECORDS=10000`, `MAX_MEMORY_CONTENT_BYTES=32MiB`
  enforced at put/import.

## Task 1: Module skeleton and record model

**Files:**
- Create: `bml/go.mod` (module `github.com/ProjectViVy/agent-vivy/bml`, `go 1.26.4`)
- Create: `bml/record.go`
- Test: `bml/record_test.go`

**Interfaces (produced):**
- `type Kind string` with consts: `KindIdentity, KindRelationship, KindCommitment, KindPreference, KindLongTerm, KindHistory, KindDaily, KindWeekly, KindMonthly, KindJournal, KindLearning, KindSessionCheckpoint, KindUnknown` — serde values `identity|relationship|commitment|preference|long_term|history|daily|weekly|monthly|journal|learning|session_checkpoint|unknown` (accept alias `working_memory` → `session_checkpoint` on unmarshal).
- `type ProvenanceSource`, `type Sensitivity`, `type Trust`, `type DigestAlgorithm`, `type EvidenceSource` — string enums, `unknown` catch-all; values per `record.rs:17-74`, `governance/types.rs:99`, `evolution/types.rs:23`.
- `type ContentDigest struct { Algorithm DigestAlgorithm; Value string }`
- `type AuditCorrelation struct { RequestID, TurnID, SessionID string; TraceID *string }` (all `json:"snake_case"`).
- `type EvidenceRef struct { ID, URI string; Source EvidenceSource; Excerpt, Hash *string; CreatedAt time.Time }`
- `func IsPrimaryGovernanceEvidence(e EvidenceRef) bool` — `e.Source != EvidenceSourceContextCompaction`.
- `type Scope struct { TenantID, WorkspaceID string; SessionID *string }`
- `type Provenance struct { Source ProvenanceSource; SourceID string; ContentDigest ContentDigest; CapturedAt time.Time; Correlation AuditCorrelation }`
- `type Tombstone struct { TargetRecordID, ActorID string; ReasonDigest ContentDigest; CreatedAt time.Time }`
- `type Record struct { ID string; Kind Kind; Content string; Provenance Provenance; EvidenceRefs []EvidenceRef; ConfidenceBPS uint16; Sensitivity Sensitivity; Trust Trust; Scope Scope; CreatedAt, EffectiveAt time.Time; ExpiresAt *time.Time; Supersedes []string; Tombstone *Tombstone }` — JSON names = serde names.
- `type ValidationError struct { Code string; Field string }` implementing `error`; codes = `record.rs:156-191` snake_case variant names.
- `const MaxConfidenceBPS uint16 = 10_000`
- `func (r *Record) ValidateAt(now time.Time, skew time.Duration) *ValidationError` — port `validate_at` at `record.rs:195-...` including digest match, future-creation skew, expiry ordering, self-supersede, tombstone-content, authority-source bans, workspace check.

- [ ] **Step 1: Write the failing test** — port the `record.rs` `#[cfg(test)]` cases at lines 387-627 into `record_test.go` table tests.
- [ ] **Step 2:** `cd bml && go test ./...` → FAIL (no code).
- [ ] **Step 3:** Implement `record.go`. Time format: RFC3339Nano UTC text in JSON/SQLite (`chrono::Utc` serializes RFC3339). Digest helper: `func MemoryContentDigest(b []byte) ContentDigest` = sha256 hex (`memory_content_digest` in core).
- [ ] **Step 4:** `go test ./...` → PASS.
- [ ] **Step 5:** Commit `feat(bml): record model and validation`.

## Task 2: Typed store open + schema + identity

**Files:**
- Create: `bml/store.go`
- Test: `bml/store_test.go`

**Interfaces (produced):**
- `type Store struct` (exported opaque). `func Open(ctx context.Context, dir, workspaceID string) (*Store, error)` — creates `{dir}/memory.sqlite3` if absent; `func OpenExisting(...)` — must exist.
- `func (s *Store) Path() string`; `func (s *Store) Close() error`.
- `type StoreError` typed errors with codes: `invalid_record`, `workspace_mismatch`, `database_workspace_mismatch`, `unsupported_schema`, `store_revision_conflict`, `record_revision_conflict`, `capacity_exceeded`, `fts_unavailable`, `io_error` (from `TypedMemoryStoreError` at `typed_store.rs:37-90`).
- `type StoreMetadata struct { SchemaVersion, StoreRevision, RecordCount, ContentBytes int64; WorkspaceID string }`; `func (s *Store) Metadata(ctx) (StoreMetadata, error)`.
- `const MaxMemoryRecords int64 = 10_000`, `const MaxMemoryContentBytes int64 = 32*1024*1024`, `const schemaVersion int64 = 1`, `const component = "embedded_laputa"`.

- [ ] **Step 1: failing test** — open fresh dir creates DB with all 5 tables; reopen works; wrong workspaceID errors `database_workspace_mismatch`; FTS5 present (query `memory_fts`).
- [ ] **Step 2:** run → FAIL.
- [ ] **Step 3:** Implement: DDL verbatim from `typed_store.rs:494-560`; one tx; insert/verify `schema_meta` row; busy_timeout 5s; WAL.
- [ ] **Step 4:** `go test` → PASS. **Step 5:** commit `feat(bml): typed store open and schema`.

## Task 3: CRUD, revisions, tombstones, capacity, supersede index

**Files:**
- Modify: `bml/store.go`
- Test: `bml/store_test.go`

**Interfaces (produced):**
- `type StoredRecord struct { Record Record; Revision int64 }` (`StoredMemoryRecord`, `typed_store.rs:96`).
- `func (s *Store) Get(ctx, id string) (*StoredRecord, error)`, `List(ctx, limit uint32) ([]StoredRecord, error)`, `SupersededTargetIDs(ctx) (map[string]struct{}, error)` (`typed_store.rs:658`).
- `func (s *Store) Put(ctx, rec Record, expectedStoreRevision int64, expectedRecordRevision *int64) (StoredRecord, error)` — port `put` at `typed_store.rs:895`: validate record, workspace check, revision checks, FTS upsert, `memory_supersedes` rows, meta update — one tx.
- `func (s *Store) PutTombstone(ctx, rec Record, expectedStoreRevision int64, targetID string, targetRevision int64) error` — `put_tombstone` at `:913`.
- `func (s *Store) ImportRecords(ctx, recs []Record) (int, error)` — `import_records` at `:763` (batch, capacity-aware).

- [ ] Steps: port Rust unit/behavior expectations as Go tests first (put→get→list roundtrip, revision conflict on stale `expectedRecordRevision`, tombstone hides content, supersedes index, capacity exceeded at limit), then implement, `go test` green, commit `feat(bml): record CRUD with revisions and tombstones`.

## Task 4: FTS5 search

**Files:**
- Modify: `bml/store.go` (or `bml/search.go`)
- Test: `bml/search_test.go`

**Interfaces (produced):**
- `type SearchHit struct { Record StoredRecord; Rank float64 }` (`MemorySearchHit`, `:103`).
- `type SearchQuery struct { Text string; Scope Scope; IncludeTombstoned bool; Limit uint32 }` — port `search`/`search_visible` (`typed_store.rs:1253`,`:1299`): FTS5 MATCH with properly quoted/sanitized query text, scope filters, tombstone/supersede visibility rules.

- [ ] Steps: failing tests (hit ranking, scope filter, tombstoned excluded, query with special FTS5 chars doesn't error), implement, green, commit `feat(bml): FTS5 search`.

## Task 5: Maintenance — GC, integrity, backup/restore, canonical identity

**Files:**
- Modify: `bml/store.go` (or `bml/maintenance.go`)
- Test: `bml/maintenance_test.go`

**Interfaces (produced):**
- `func (s *Store) GCSessionScoped(ctx, sessionID string) (uint64, error)` (`:686`), `GCStaleSessionScoped(ctx, olderThan time.Duration) (uint64, error)` (`:715`).
- `func (s *Store) Integrity(ctx) (IntegrityReport, error)` (`:1340`) — port `MemoryIntegrityReport`/`MemoryIntegrityFinding` from `record.rs:125-151`.
- `func (s *Store) Backup(ctx, dest string) error` / `Restore(ctx, backup string) (*Store, error)` (`:1405`,`:1420`).
- `func OpenCanonical(ctx, dir, workspaceID string)` / `OpenExistingCanonical` / `RollbackCanonicalIdentity` (`:220`,`:233`,`:345`) — canonical identity migration + `WorkspaceIdentityMigrationManifest`/`State` (`:124-145`).

- [ ] Steps: failing tests per function group, implement, green, commit `feat(bml): gc, integrity, backup/restore, canonical identity`.

## Task 6: MemoryHome facade

**Files:**
- Create: `bml/home.go`, `bml/atomic.go`
- Test: `bml/home_test.go`

**Interfaces (produced):**
- `type Home struct` — `NewHome(configDir string) *Home`, `NewHomeWithL1Budget(configDir string, l1IndexLines int) *Home` (`DEFAULT_L1_INDEX_LINES = 30`).
- Path accessors: `ConfigDir()`, `MemoryDir()` = `{config}/memory`, `DatabasePath()` = `{dir}/memory.sqlite3`, `MemRulesPath()` = `{config}/memory/MEMRULES.MD` — verify exact paths from `memory_home.rs:103-145` and `layout.rs`.
- `func (h *Home) Warmup(ctx) error` (`:149`).
- `func (h *Home) ListRecords(ctx, limit uint32) ([]StoredRecord, error)`, `GetRecord(ctx, id string) (*StoredRecord, error)` — nil when store absent; filter `visible_long_term` (`memory_home.rs` helper — port verbatim).
- `func (h *Home) AddRecord(ctx, content string, evidence []EvidenceRef) (StoredRecord, error)` — kind forced `long_term`, forbidden otherwise; ID format `memory-{unix_micros}-{digest[:12]}`; provenance `user_input`/`memory_add` (verify exact strings at `:205-237`); refresh startup index (port `refresh_startup_index` and its L1 index file render — `render_l1_index_block` in core `memory/`).
- `func (h *Home) UpdateRecord(ctx, id, content string, baseRevision int64, evidence []EvidenceRef) (StoredRecord, error)` — port `:238-282` incl. not-found mapping and provenance rewrite (`user_input`/`memory_update`).
- `func (h *Home) RemoveRecord(ctx, id, reason string, baseRevision int64) (StoredRecord, error)` — port `:284-346`: tombstone record `memory-tombstone-{unix_micros}`.
- `type MemRulesDocument struct { Content string; Source MemRulesSource }`, `MemRulesSource{Default,File}`; `ReadMemRules()/WriteMemRules(content)` (`:348-373`) via `atomicWrite` (port `atomic.rs`).
- `func (h *Home) ClearSessionCheckpoint(ctx, sessionID string) (uint64, error)` (`:374`), `RunStartupGC(ctx)` (`:381` — read the full body for stale-session policy).
- `type HomeError` with `Code() string` per Global Constraints.

- [ ] Steps: failing tests for each facade method incl. error codes; implement; green; commit `feat(bml): MemoryHome facade`.

## Task 7: Offline migration adapters

**Files:**
- Create: `bml/migrate.go`
- Test: `bml/migrate_test.go`

**Interfaces (produced):**
- `func AdaptLegacyMarkdown(...)`, `func AdaptLaputaSection(...)`, `func CompareNormalizedRecords(...)` — port signatures verbatim from `memory_records.rs:66,108,163`.
- `type MemoryMigration` with `DryRun`, `Execute`, `Rollback` + `MemoryMigrationPlan`, `MemoryMigrationManifest`, `MemoryRollbackManifest`, `MemoryMigrationTestFailure`, `MemoryAdapterContext`, `MemoryAdapterOutput` (`:163-360`).

- [ ] Steps: port `memory_records.rs` test cases (lines 482-702) to `migrate_test.go`; implement; green; commit `feat(bml): legacy memory migration adapters`.

## Task 8: Package docs + integration hooks

**Files:**
- Create: `bml/README.md` (English: scope, what's ported vs deferred, how to run tests)
- Modify: none in root (root `just ci` intentionally untouched; wiring decision belongs to integration Story)

- [ ] Write README; run `cd bml && gofmt -l . && go vet ./... && go test ./...`; commit `docs(bml): module readme`.
