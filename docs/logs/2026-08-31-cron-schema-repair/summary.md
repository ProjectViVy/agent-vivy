# Cron schema repair

## What changed

- Added SQLite `migration017`, an idempotent repair for Journals that already
  recorded migration 016 before the channel and cron branches were merged but
  never received the `cron_jobs` table.
- Added a SQLite regression test that removes the table while retaining the
  migration-016 marker, reopens the Journal, and verifies `ListCronJobs`
  succeeds after the repair migration.
- Advanced the Postgres schema marker to version 16 and added a compatible
  repair migration. It reconciles both historical version-15 shapes by
  conditionally ensuring message provenance columns and the `cron_jobs` table.
- Added the Postgres upgrade assertion for the repaired cron table.

## Scope explicitly not changed

- The scheduler, cron expression parser, RPC surface, and UI behavior were not
  changed.
- No tenant Journal under `data/` was opened, modified, or deleted.
