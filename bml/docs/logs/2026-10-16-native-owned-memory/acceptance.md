# Native acceptance boundary

- An exact native operation commits its canonical row/tombstone, ownership,
  immutable outcome, FTS and counters together. Faults acknowledge no outcome and
  leave no partial state. Replay returns the original actual result after later
  corrections, deletion and reopen.
- Changed body, source, CAS shape, digest, scope or destination cannot reuse an
  operation identity or disclose another owner. Raw invalid UTF-8 cannot be
  normalized into the fingerprint. Unknown old records remain legacy only.
- Legacy CRUD/recall/List/L1/GC exclude owned rows before limits; exact bound
  search/export excludes legacy rows, stale revisions and removed material.
- A supported old database upgrades through the actual Home path without
  rewriting original records or inventing ownership/receipts. Read-only databases
  and backups remain unchanged. Failed migration/restore preserves prior state.
- Bound data/history prevents canonical workspace identity rewriting before any
  metadata/backup write. Ordinary unbound legacy migration still works.

These are disposable native SQLite fixtures. They do not establish actual Windows,
App/worker/C2 qualification, adapter conformance or installed-product acceptance.
