# Summary

Issue #41 centralizes Vivy's core schema ownership under
`internal/storage/migrations`.

Implemented:

- Paired SQLite/PostgreSQL logical migrations 001–023 as embedded SQL files.
- Manifest validation for ordering, names, dialect pairs, and checksums.
- One shared `database/sql` runner with metadata backfill, checksum drift
  detection, and one transaction per migration.
- PostgreSQL legacy marker normalization based on historical markers plus a
  verified contiguous schema shape, including repair of the historical
  truncation-table gap.
- SQLite and PostgreSQL backend delegation to the shared runner.
- Repository rules and authoring documentation prohibiting hidden DDL and raw
  plugin access to the core database.

Not included:

- An external migration framework or ORM.
- A public plugin database API.
- Any change to storage contracts or runtime lease behavior.
