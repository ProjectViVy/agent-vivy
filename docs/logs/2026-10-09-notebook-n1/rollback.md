# N1 rollback

- No destructive changes: `notes` table retained as archival; migration 036
  only adds tables.
- Rollback = revert commit; head-035 fixtures skip the new tests
  (`seedHead35` pattern). Migration files are append-only per repo rules.
