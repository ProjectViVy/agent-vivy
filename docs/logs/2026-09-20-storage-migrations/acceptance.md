# Acceptance

A reviewer can verify the change by checking:

1. `internal/storage/migrations` contains paired 001–023 SQL files and the
   executable embeds them.
2. `sqlite.Open` and `postgres.Open/OpenSchema` call the same runner and no
   backend Go file owns production migration bodies.
3. Reopening a SQLite database is a no-op, while removing the 017 marker from
   the historical 016-only shape recreates the cron table and metadata.
4. Changing an applied checksum fails startup before new migrations run, and a
   deliberately failing migration leaves no table or marker behind.
5. A PostgreSQL v14 fixture is normalized to canonical 001–023 markers when a
   PostgreSQL test DSN is available; the upgraded schema and a fresh schema
   expose the same logical table/column contract.
6. `AGENTS.md` and `docs/architecture/STORAGE-MIGRATIONS.md` make Core Storage
   ownership, append-only authoring, embedded delivery, and plugin boundaries
   explicit.
