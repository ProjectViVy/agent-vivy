# bml — Basic Memory Layer

`bml` is a standalone Go port of Diva's BML (Basic Memory Layer): the
machine-wide typed SQLite + FTS5 memory authority that lives at
`{config_dir}/memory/memory.sqlite3` and is exposed through the `Home`
facade.

It is its own Go module (`github.com/ProjectViVy/agent-vivy/bml`) so it can be
extracted later. It does not import any `agent-vivy` root packages, and the
root module does not import it — wiring into the Vivy runtime is a separate
integration story.

Source of truth for the port: `agent-diva` at `0fd005a1`
(`agent-diva-core` + `agent-diva-laputa` crates).

## Package layout

| File | Ports | Contents |
| --- | --- | --- |
| `record.go` | `agent-diva-core/src/memory/record.rs`, `governance/types.rs`, `evolution/types.rs` | Normalized `Record` model, enums (`Kind`, `ProvenanceSource`, `Sensitivity`, `Trust`, `DigestAlgorithm`, `EvidenceSource`), `Provenance`/`Scope`/`Tombstone`/`EvidenceRef`/`ContentDigest`/`AuditCorrelation`, `ValidateAt`, `MemoryContentDigest` |
| `store.go` | `agent-diva-laputa/src/typed_store.rs` | `Store` open/schema/identity, CRUD (`Get`/`List`/`Put`/`PutTombstone`/`ImportRecords`), store + per-record revision checks, supersede index, capacity limits, typed `StoreError` codes |
| `search.go` | `typed_store.rs` `search`/`search_visible` | FTS5 `Search`/`SearchVisible` with scope filters and tombstone/supersede visibility |
| `maintenance.go` | `typed_store.rs` maintenance section | `GCSessionScoped`, `GCStaleSessionScoped`, `Integrity`, `Backup`/`Restore`, canonical workspace identity (`OpenCanonical`, `OpenExistingCanonical`, `RollbackCanonicalIdentity`) |
| `home.go` | `agent-diva-laputa/src/bml/memory_home.rs` | `Home` facade: lazy store handles, revisioned long-term CRUD, `MEMRULES.MD` read/write, session GC, startup L1 index projection, typed `HomeError` codes |
| `migrate.go` | `agent-diva-laputa/src/memory_records.rs` | Offline migration adapters: `AdaptLegacyMarkdown`, `AdaptLaputaSection`, `CompareNormalizedRecords`, `MemoryMigration` (`DryRun`/`Execute`/`Rollback`) with manifests |
| `atomic.go` | `agent-diva-laputa/src/atomic.rs` | `atomicWrite`/`atomicWriteJSON` (tmp + fsync + rename) |

## Ported

- The Memory v2 record model with serde-compatible `snake_case` JSON; the
  on-disk `record_json` bytes are byte-identical to `serde_json::to_string`.
- The full SQLite schema byte-faithful from `typed_store.rs`
  (`schema_meta`, `memory_records`, `memory_supersedes`,
  `memory_apply_journal`, `memory_fts` with `tokenize='unicode61'`), verified
  at open time including FTS5 availability.
- Optimistic concurrency: `store_revision` + per-record `record_revision`
  checks, `*_revision_conflict` error codes.
- Tombstones (content never resurfaces via get/list/search), workspace
  binding (`database_workspace_mismatch`), capacity limits
  (`MaxMemoryRecords = 10_000`, `MaxMemoryContentBytes = 32 MiB`).
- FTS5 search with MATCH-query quoting, ranking, scope and visibility rules.
- GC (session-scoped and stale-session), integrity report, backup/restore,
  and the canonical workspace-identity migration with manifests under
  `{dir}/migrations/workspace-identity-v1/`.
- The `Home` facade surface used by callers today: warmup, long-term
  add/update/remove/get/list, MEMRULES document handling, session-checkpoint
  GC, startup GC, startup markdown/L1 index rendering.

## Dropped upstream (not ported, intentionally)

- **Governed write seam** — `put_governed`, `rollback_governed`, and
  `GovernedMemoryApply` are dead code upstream (the cognitive clean break
  removed the governed-apply pipeline; see `bml/mod.rs`). They are not
  ported. The `memory_apply_journal` table is kept in the schema for byte
  compatibility.

## Deferred (out of scope for this module)

- **ACTMEM** — `actmem.rs` (the working-memory markdown capsule store) and
  the `MemoryHome.actmem()` accessor.
- **Diva `MemoryProvider` trait surface** — `prefetch`, `sync_turn`,
  `checkpoint`, `session_end`, `system_prompt`, `recall_outcome`,
  `actmem_*`. These are runtime assembly, not BML storage. Consequently the
  `checkpoint_id`/`write_checkpoint` session-checkpoint write path inside
  `MemoryHome` is also unported; only session-checkpoint GC
  (`ClearSessionCheckpoint`/`RunStartupGC`) is present.
- **Runtime wiring** — no Vivy `Port`/`Module`/provider glue; the root module
  does not depend on `bml` yet.

## Known divergences

- **Sync, not async.** The Rust API is `async` over `tokio`/`sqlx`. This port
  is synchronous `database/sql`: every fallible method takes a
  `context.Context` first parameter, and read-modify-write sequences are
  serialized with a `sync.Mutex` instead of pool-level locking.
- **`json.MarshalIndent` HTML-escaping.** `atomicWriteJSON` uses
  `json.MarshalIndent`, which escapes `<`, `>`, `&` (and emits
  `\u2028`/`\u2029`) where `serde_json::to_string_pretty` emits literal
  characters. Pretty-printed artifacts (migration manifests, memrules-side
  JSON) may therefore differ byte-for-byte from Rust output. This does not
  affect `record_json`: canonical record bytes go through `marshalCanonical`,
  which disables HTML escaping and restores raw U+2028/U+2029 to match serde
  exactly.
- **Error types are Go-typed** (`StoreError`, `HomeError`, `MigrationError`,
  `ValidationError`) but expose the same machine-readable codes as the Rust
  originals, e.g. `database_workspace_mismatch`, `store_revision_conflict`,
  `bml_unavailable`, `memory_kind_forbidden`, `memory_not_found`.
- Errors never embed memory `Content` bytes in messages, matching the Rust
  redaction contract.

## Build & test

Requires Go 1.26.4+.

```sh
cd bml
go test ./...
```

Lint/format gates (must be clean):

```sh
cd bml
gofmt -l .        # no output
go vet ./...
```

## Dependencies

- `modernc.org/sqlite` **v1.56.0** — pure-Go (CGO-free) SQLite driver,
  registered under the driver name **`sqlite`**; same version pinned by the
  root `go.mod`. Everything else is the Go standard library.
- The store opens SQLite with WAL journal mode, `foreign_keys(1)`, and a
  5 s busy timeout, via `database/sql` URI pragmas.
