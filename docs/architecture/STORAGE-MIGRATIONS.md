# Storage Migration Workflow

Core Storage owns the database schema for every first-party backend. The
release binary embeds the complete migration catalog through `go:embed`; it
does not read an external SQL directory at runtime.

## Layout and naming

```text
internal/storage/migrations/
├── manifest.go
├── runner.go
├── metadata/
│   ├── sqlite.sql
│   └── postgres.sql
├── sqlite/
│   └── NNN_name.sql
└── postgres/
    └── NNN_name.sql
```

Each logical migration has one file per dialect. `NNN` is a three-digit,
one-based, contiguous version and `name` is lower snake case. The SQLite and
PostgreSQL filenames must have the same version and stem. SQL bodies may differ
when the dialect requires it.

## Authoring a migration

1. Add the next version to both dialect directories; never renumber, delete, or
   rewrite a released migration.
2. Keep all core DDL in these files. Do not add schema creation to a repository,
   feature package, plugin, or `init()` function.
3. Make the SQL safe for the databases that can reach that version. If a
   historical backend needs a compatibility repair, keep it in the paired
   versioned file and document the shape it repairs.
4. Add or update fresh-install, upgrade, reopen/no-op, failure-rollback, and
   dialect-parity tests. Existing data must survive an upgrade.
5. Run the focused storage tests, `go vet ./...`, the complete Go test suite, and
   `just ci` when the repository UI/toolchain dependencies are available.

The manifest validates duplicate and missing versions, invalid filenames,
missing dialect pairs, and logical name drift before applying production DDL.
Final metadata contains exactly `version/name/applied_at`. The active manifest
and runner do not compute, backfill or validate file checksums. Released SQL
remains immutable; add a paired migration for any schema change.

The runner initializes a truly pristine database in one transaction: historical
DDL, all current owner migrations and final metadata commit together. Later
known compatible target migrations use one transaction per migration with its
marker. Startup inspects metadata, contiguous known names/versions and the
required actual target tables/columns before any write. Old metadata returns
`upgrade_required`; partial or unknown formats return `unsupported_source_format`.

Historical normalization, released-017 repair and conversion operate on copies
in a separately delivered finite tool. They are absent from the runtime writer.
The supported source matrix is [design §§11.7–11.9](../plans/2026-10-19-complete-deletion-plan.md#117-final-schemas-and-finite-supported-source-formats).
Current D03 migration036 is an integrated candidate barrier; it is not D14's
final target number, and old-lineage036_notebook_content is unsupported.

Plugins do not receive raw core database access. Plugin persistence must use an
explicit public Storage Port with an independently reviewed contract.
