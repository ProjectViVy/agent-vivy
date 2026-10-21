# bml — Basic Memory Layer

`bml` is the independent Go module behind Vivy's first-party memory Module.
It owns bounded SQLite/FTS5 memory records, workspace ownership, record/store
revisions, tombstones, search, backup/restore and the machine-home facade.
Requires Go 1.26.4 and `modernc.org/sqlite` v1.56.0.

## Active formats

The active database schema is **2** and import artifact format is **2.0.0**.
Record JSON has no provenance content digest, tombstone reason digest or
evidence hash. Existing opaque IDs, scopes, links, revisions and tombstones
survive conversion; new IDs are random opaque allocations. The historical
Diva port (`agent-diva@0fd005a1`) is provenance, not a byte-compatibility
requirement for these deliberately changed formats.

A store contains `schema_meta`, `memory_records`, `memory_supersedes` and
`memory_fts` (FTS5 with the unicode61 tokenizer). The obsolete governed-write
`memory_apply_journal` is absent. Store and per-record revisions retain CAS;
workspace mismatches, capacity limits, trust rules and search visibility
remain authoritative.

`OpenCanonical` reads persisted workspace identity. Fresh homes allocate an
identity once; moving a home preserves it. Callers requiring a particular
workspace use `Open`/`OpenExisting` with that expected owner. The machine-home
facade retains the fixed `machine-memory-home` owner.

Existing stores are inspected read-only before WAL pragmas, initialization or
repair. Recognized schema1 requires a separately delivered finite converter.
Unknown, partial and future shapes are refused before writes. The final runtime
has one active record parser; it never upgrades a live legacy home implicitly.
See issue-91 D12 and design §§11.7–11.9 for supported source/conversion rules.

## Imports and rollback

`AdaptLegacyMarkdown` and `AdaptLaputaSection` normalize sources without
modifying them. New normalization allocates opaque IDs. `MemoryMigration`
keeps exactly three artifact files: records.json, rollback.json and manifest.json.
The manifest privately owns exact raw input, admitted normalized records and
source-to-record allocations; records.json is generated from that admission
and checked for equality on completed replay. These files contain private data.

`DryRun` never writes. `Execute` claims a fresh artifact directory and records
admitted values/allocations before writing projections. An interrupted import
retains that admission; restart reuses its IDs and rejects changed raw or
normalized inputs. Unexplained partial directories cannot be adopted.
Completed old artifacts converted without recoverable input are marked
`unverifiable`; their migration IDs cannot be reused.

Rollback accepts only the known owned output list, refuses linked migration
directories, and never deletes source memory or another home. Database conversion
operates on copies; an old executable must not open a converted database.

## Home and MEMRULES

`Home` lazily opens the machine store and exposes revisioned long-term CRUD,
FTS recall, history projection, session GC and a bounded startup index.
`CompareAndWriteMemRules` compares exact previously read text and file existence
under the Home writer mutex before normalizing the proposed replacement.
Missing default rules differ from an existing file with identical text.

The memory action/UI contract uses `expected_content` and `expected_exists`;
reads return `content`, `source` and `exists`. A stale write keeps the draft
and requires a refresh. No digest token or rules digest is returned in status.

## Verification

```sh
cd bml
go test ./... -count=1
go vet ./...
gofmt -l .
```

Tests use disposable stores. Root-module tests do not include this independent
module. Product/upgrade acceptance and unrun lanes are recorded in the D12
iteration log; a passing module suite alone does not accept the full data lane.
