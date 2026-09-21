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
│   ├── postgres.sql
│   └── *_add_metadata.sql
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
missing dialect pairs, and name drift before applying production DDL. Every
applied row stores the logical name and SHA-256 checksum. Editing an applied
file is a checksum-drift failure; create a new append-only migration instead.

The runner applies each pending migration in its own `database/sql` transaction
and records its marker in that transaction. A failed migration therefore aborts
startup without exposing a partial schema or marker. Legacy databases receive a
one-time metadata backfill; the old PostgreSQL compressed marker sequence is
normalized using both its marker history and verified schema shape.

Plugins do not receive raw core database access. Plugin persistence must use an
explicit public Storage Port with an independently reviewed contract.
