# Centralized Storage Migrations Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Move SQLite and PostgreSQL production schema DDL into paired embedded SQL migrations with one validated, checksummed, transactional runner while preserving existing database upgrades.

**Architecture:** `internal/storage/migrations` owns the embedded migration catalog, metadata table bootstrap, manifest validation, checksum verification, legacy marker normalization, and per-migration transaction loop. SQLite and PostgreSQL backends only select the dialect and pass their existing `*sql.DB`; they retain their lease and storage contracts. The canonical logical migration set is 001–023 with identical names in both dialect directories; PostgreSQL may use dialect-specific or idempotent SQL because its historical bootstrap compressed earlier logical states.

**Tech Stack:** Go standard library (`embed`, `io/fs`, `database/sql`, `crypto/sha256`), modernc.org/sqlite, pgx/stdlib, Go tests.

**Spec:** GitHub Issue #41 — `refactor(storage): centralize and embed versioned SQL migrations`.

## Global Constraints

- One schema owner, one migration path, no hidden DDL.
- Core Storage exclusively owns initialization, schema, and migrations; plugins do not access or migrate the core database directly.
- Migration files are immutable, append-only, embedded at build time, and require no runtime SQL directory.
- SQLite and PostgreSQL expose the same logical migration IDs and names; SQL may differ by dialect.
- No ORM, external migration framework, or new database dependency.
- Preserve fail-closed startup, existing storage contracts, and one transaction per migration.
- Follow More / Fast / Good / Frugal: no speculative provider API, scheduler, or storage abstraction beyond the issue contract.

## Review Focus

- A legacy SQLite database with a missing repair marker must receive only the missing migration and its metadata, not rerun applied DDL.
- A legacy PostgreSQL database whose old marker 15–21 represents compressed schema states must normalize to the canonical logical state before applying new files.
- Duplicate, gapped, malformed, or cross-dialect-mismatched migration files must fail validation before any application migration runs.
- A migration that fails partway must leave neither its DDL nor its `schema_migrations` row committed.
- Fresh and upgraded databases must converge on the same logical table/column/index contract, and re-opening must be a no-op.

### Task 1: Add the paired embedded migration catalog

**Files:**
- Create: `internal/storage/migrations/manifest.go`
- Create: `internal/storage/migrations/manifest_test.go`
- Create: `internal/storage/migrations/sqlite/001_initial.sql` through `023_workspace_path.sql`
- Create: `internal/storage/migrations/postgres/001_initial.sql` through `023_workspace_path.sql`
- Create: `internal/storage/migrations/metadata/sqlite.sql`
- Create: `internal/storage/migrations/metadata/postgres.sql`
- Create: `internal/storage/migrations/metadata/sqlite_add_metadata.sql`
- Create: `internal/storage/migrations/metadata/postgres_add_metadata.sql`

**Interfaces:**
- Produces `migrations.Dialect`, `migrations.Migration`, `migrations.Manifest`, `migrations.Load(fs.FS)`, `migrations.Embedded()`, and `migrations.ValidateFS(fs.FS)` for the runner and backend tests.
- `Manifest.Migrations(dialect)` returns migrations ordered by numeric version and exposes immutable name/checksum values.

- [ ] **Step 1: Write the failing manifest tests**

  Add tests for a valid paired `fstest.MapFS`, duplicate version, missing version gap, invalid filename, missing dialect pair, and mismatched dialect name. Assert errors name the offending path/version so CI diagnostics are actionable.

- [ ] **Step 2: Run the manifest tests to verify they fail for the missing catalog API**

  Run: `/workspace/scratch/4f137d903ad1/toolchains/go1.26.4/bin/go test ./internal/storage/migrations -run 'TestManifest' -count=1`

  Expected: FAIL because `internal/storage/migrations` and its validation API do not exist yet.

- [ ] **Step 3: Implement the minimal catalog parser and validator**

  Use `fs.ReadDir` and `fs.ReadFile`, require `NNN_lower_snake_case.sql`, reject duplicate/gapped versions, require identical version/name sets for `sqlite` and `postgres`, and calculate a SHA-256 checksum from the exact embedded bytes. Keep the embedded `fs.FS` private and expose only read-only catalog values.

- [ ] **Step 4: Extract SQLite DDL mechanically into named files and create paired PostgreSQL files**

  Preserve each SQLite migration body byte-for-byte from the current `migration001`–`migration023` constants. Rename the PostgreSQL full bootstrap to `001_initial.sql`; use idempotent dialect-specific files for logical versions 016–023 and no-op/idempotent compatibility files where the PostgreSQL bootstrap already contains an earlier SQLite-era shape. Every PostgreSQL filename must use the exact SQLite logical name.

- [ ] **Step 5: Add the metadata SQL files**

  `metadata/{sqlite,postgres}.sql` creates `schema_migrations(version, name, checksum, applied_at)`; the `*_add_metadata.sql` files add `name` and `checksum` to pre-#41 tables. Keep metadata DDL in embedded SQL files rather than Go string constants.

- [ ] **Step 6: Run the manifest tests to verify they pass**

  Run: `/workspace/scratch/4f137d903ad1/toolchains/go1.26.4/bin/go test ./internal/storage/migrations -run 'TestManifest' -count=1`

  Expected: all manifest validation tests PASS, including the real embedded 001–023 catalog.

- [ ] **Step 7: Commit the catalog**

  ```bash
  git add internal/storage/migrations
  git commit -m "refactor(storage): add paired embedded migration catalog"
  ```

### Task 2: Implement the shared runner and SQLite behavior contract

**Files:**
- Create: `internal/storage/migrations/runner.go`
- Create: `internal/storage/migrations/runner_test.go`
- Modify: `internal/storage/migrations/manifest.go`

**Interfaces:**
- Consumes `Manifest` from Task 1 and a dialect-selected `*sql.DB`.
- Produces `migrations.Apply(ctx, db, dialect) error` and `migrations.ApplyManifest(ctx, db, dialect, manifest) error`.

- [ ] **Step 1: Write failing runner tests**

  Use a real temporary SQLite database and test: fresh apply records all versions/names/checksums; reapply is a no-op; a checksum change fails closed; a failing custom migration rolls back its table and marker; and an old metadata table is upgraded and backfilled. Use `ApplyManifest` for the failing custom catalog so the test exercises the real transaction loop.

- [ ] **Step 2: Run the runner tests to verify the expected failures**

  Run: `/workspace/scratch/4f137d903ad1/toolchains/go1.26.4/bin/go test ./internal/storage/migrations -run 'Test(Apply|Runner|Legacy)' -count=1`

  Expected: FAIL because the shared runner and metadata handling do not exist.

- [ ] **Step 3: Implement metadata setup without Go-embedded DDL**

  Load the dialect metadata files from the embedded catalog, create `schema_migrations` when absent, inspect existing columns, and apply the legacy metadata SQL only when `name`/`checksum` are missing. Reject partially upgraded metadata instead of guessing.

- [ ] **Step 4: Implement per-migration validation and transaction application**

  Before applying production DDL, validate the catalog and read all applied rows. For each existing row require matching version/name/checksum; for each pending migration begin a fresh `sql.Tx`, execute its SQL, insert the marker with the exact checksum, and commit. Roll back on every execution or marker error and return a fail-closed error. Use `?` for SQLite and `$n` for PostgreSQL marker queries.

- [ ] **Step 5: Implement legacy SQLite metadata backfill**

  For a pre-#41 SQLite table, fill name/checksum only from the canonical manifest for already-recorded versions. Do not change the version set or rerun their SQL. A missing version remains pending and is applied through the normal transaction loop.

- [ ] **Step 6: Run the focused runner tests to verify they pass**

  Run: `/workspace/scratch/4f137d903ad1/toolchains/go1.26.4/bin/go test ./internal/storage/migrations -run 'Test(Apply|Runner|Legacy)' -count=1`

  Expected: all runner behavior tests PASS, including rollback and checksum drift.

- [ ] **Step 7: Commit the shared runner**

  ```bash
  git add internal/storage/migrations
  git commit -m "refactor(storage): add checksummed migration runner"
  ```

### Task 3: Preserve PostgreSQL historical compatibility

**Files:**
- Modify: `internal/storage/migrations/runner.go`
- Create: `internal/storage/migrations/postgres_legacy_test.go`
- Modify: `internal/storage/postgres/upgrade_test.go`

**Interfaces:**
- Consumes the canonical manifest and shared runner from Tasks 1–2.
- Produces a legacy normalization path internal to `migrations`; `postgres.Open` does not own a second migration algorithm.

- [ ] **Step 1: Write the failing legacy-normalization tests**

  Test the old PostgreSQL marker plus observed schema-shape mapping for v14, the accumulated v15–18 bootstrap path, and the v19/v20/v21 upgrade path. Assert that the canonical applied state is derived from the strongest verified table/column shape (a full accumulated bootstrap becomes logical 23 even when its old marker is 15), with canonical names/checksums. Include an unsupported future marker and partial metadata case that must fail closed.

- [ ] **Step 2: Run the tests to verify they fail before the compatibility path exists**

  Run: `/workspace/scratch/4f137d903ad1/toolchains/go1.26.4/bin/go test ./internal/storage/migrations -run 'TestPostgresLegacy' -count=1`

  Expected: FAIL because legacy PostgreSQL marker normalization is not implemented.

- [ ] **Step 3: Implement legacy PostgreSQL normalization**

  Detect a pre-#41 `schema_migrations` table by missing metadata columns. Within a metadata transaction, map the old compressed sequence to the canonical logical state, replace old markers with canonical 001..state rows, and preserve fail-closed behavior for unsupported/future shapes. Use the existing `session_compactions` shape check for old version 14 so the v14 upgrade fixture becomes logical 15 before applying 016 onward.

- [ ] **Step 4: Update the PostgreSQL upgrade test to assert the canonical history and equivalent schema**

  Keep the frozen v14 fixture as a historical input, run the production `OpenSchema` path, assert canonical 001–023 markers, legacy row survival, provenance defaults, cron table, and workspace column. Add a fresh-vs-upgraded logical schema comparison for the columns/tables used by the storage conformance contract.

- [ ] **Step 5: Run the focused PostgreSQL tests**

  Run: `/workspace/scratch/4f137d903ad1/toolchains/go1.26.4/bin/go test ./internal/storage/postgres -run 'TestMigrateUpgradesV14InPlace' -count=1`

  Expected: the test remains skipped when `VIVY_POSTGRES_TEST_DSN` is unset; when a PostgreSQL DSN is provided it exercises the full compatibility path and passes. Run the static migration/normalization tests unconditionally.

- [ ] **Step 6: Commit the PostgreSQL compatibility path**

  ```bash
  git add internal/storage/migrations internal/storage/postgres/upgrade_test.go
  git commit -m "refactor(storage): normalize legacy postgres migration markers"
  ```

### Task 4: Switch both storage backends to the shared owner

**Files:**
- Modify: `internal/storage/sqlite/sqlite.go`
- Modify: `internal/storage/postgres/postgres.go`
- Modify: `internal/storage/sqlite/sqlite_test.go`
- Modify: `internal/storage/postgres/upgrade_test.go`

**Interfaces:**
- Consumes `migrations.Apply` from Task 2 and its legacy normalization from Task 3.
- Preserves `sqlite.Open`, `postgres.Open`, `postgres.OpenSchema`, lease acquisition, and all storage contract implementations.

- [ ] **Step 1: Add backend integration assertions before removing the old runners**

  Add SQLite assertions that `schema_migrations` contains canonical names/checksums and that the migration016-only repair shape still repairs on reopen. Add a source-level guard test or CI check that production backend Go files contain no migration DDL constants.

- [ ] **Step 2: Run the focused integration tests to establish the failing expectations**

  Run: `/workspace/scratch/4f137d903ad1/toolchains/go1.26.4/bin/go test ./internal/storage/sqlite ./internal/storage/postgres -run 'Test(Reopen|Migrate|Storage)' -count=1`

  Expected: the new metadata assertions fail against the old local runners.

- [ ] **Step 3: Replace SQLite’s local migration table and `migrate` loop**

  Delete `migrations` and `migration001`–`migration023` from `sqlite.go`, import `internal/storage/migrations`, and call `migrations.Apply(ctx, b.db, migrations.SQLite)` from `Open`. Keep the one-writer setting and all runtime storage SQL untouched.

- [ ] **Step 4: Replace PostgreSQL’s `schemaVersion` and `schemaV*` runner**

  Delete `schemaVersion`, `schemaV15`, `schemaV15Upgrade`–`schemaV21Upgrade`, and the local `migrate` implementation. Import `internal/storage/migrations` and call the shared runner on `b.db.SQL`. Keep schema-name validation, pool setup, advisory lock, lease, and fencing behavior unchanged.

- [ ] **Step 5: Remove stale production DDL files and rename references**

  Delete `internal/storage/postgres/schema.go`; update comments and tests from stale `schemaV15` terminology to canonical migration names. No repository or feature package may retain production schema DDL.

- [ ] **Step 6: Run storage integration and conformance tests**

  Run: `/workspace/scratch/4f137d903ad1/toolchains/go1.26.4/bin/go test -timeout 20m ./internal/storage/... -count=1`

  Expected: SQLite, Postgres unit tests, and conformance packages pass; Postgres DSN-gated tests may skip only with the explicit environment reason.

- [ ] **Step 7: Commit the backend switch**

  ```bash
  git add internal/storage/sqlite internal/storage/postgres
  git commit -m "refactor(storage): route backends through shared migration owner"
  ```

### Task 5: Add repository rules and authoring documentation

**Files:**
- Modify: `AGENTS.md`
- Create: `docs/architecture/STORAGE-MIGRATIONS.md`
- Create: `docs/logs/2026-09-20-storage-migrations/summary.md`
- Create: `docs/logs/2026-09-20-storage-migrations/verification.md`
- Create: `docs/logs/2026-09-20-storage-migrations/acceptance.md`

- [ ] **Step 1: Add the Database Schema Ownership rule**

  State that Core Storage is the sole owner of schema initialization/migrations; DDL must be in `internal/storage/migrations` embedded files; repositories, features, plugins, and `init()` functions must not create or migrate core tables; plugins require an explicit public Storage Port and never raw core DB access.

- [ ] **Step 2: Document the migration authoring workflow**

  Explain append-only paired filenames, naming/version rules, dialect-specific SQL, checksum drift policy, fresh/upgrade/reopen tests, and the required `go test`, `go vet`, and `just ci` gates. Include the command-independent fact that runtime artifacts need no external SQL directory.

- [ ] **Step 3: Write the iteration log from actual evidence**

  Record scope, non-goals, verification commands, PostgreSQL DSN availability, and human acceptance checks. Do not claim a PostgreSQL integration pass when the DSN is absent.

- [ ] **Step 4: Run documentation/source guards**

  Run: `rg -n 'schemaV15|const migration[0-9]+|CREATE TABLE' internal/storage/sqlite internal/storage/postgres --glob '*.go'`

  Expected: no production migration DDL or stale bootstrap names; test fixtures may retain historical SQL where explicitly required.

- [ ] **Step 5: Commit documentation and rules**

  ```bash
  git add AGENTS.md docs/architecture/STORAGE-MIGRATIONS.md docs/logs/2026-09-20-storage-migrations
  git commit -m "docs(storage): define schema ownership and migration workflow"
  ```

### Task 6: Full verification and review handoff

- [ ] **Step 1: Run formatting and static checks**

  Run: `/workspace/scratch/4f137d903ad1/toolchains/go1.26.4/bin/gofmt -w internal/storage/migrations internal/storage/sqlite internal/storage/postgres` followed by `/workspace/scratch/4f137d903ad1/toolchains/go1.26.4/bin/go vet ./...`.

- [ ] **Step 2: Run the complete backend and repository test gates**

  Run: `/workspace/scratch/4f137d903ad1/toolchains/go1.26.4/bin/go test -timeout 20m ./... -count=1` and the repository’s `just ci` equivalent when the required `pnpm`/UI dependencies are available. Capture skipped environment gates explicitly.

- [ ] **Step 3: Review the final diff against Issue #41**

  Check each acceptance criterion, confirm no new dependency or runtime SQL directory, inspect the migration manifest/file pairs, and verify the backend diff contains only delegation and storage runtime logic.

- [ ] **Step 4: Commit any verification-only fixes and report evidence**

  Do not claim completion until fresh command output, the final diff, and the iteration log agree. The branch is ready for a pull request only after the full suite and source guards are recorded.
