# Acceptance

1. Start Vivy against an existing SQLite Journal created by the pre-merge
   channel release, where `schema_migrations` already contains version 16 but
   `cron_jobs` does not exist.
2. On startup, migration 017 creates `cron_jobs` and its wake-time index
   without changing existing Journal rows.
3. The cron scheduler can list jobs immediately; the recurring
   `cron list for wake time failed` warning and `no such table: cron_jobs` error
   no longer appear.
4. Reopening the same Journal is idempotent, and a fresh Journal continues to
   create and list cron jobs normally.
