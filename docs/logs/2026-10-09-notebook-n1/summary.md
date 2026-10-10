# N1 — Scoped revisioned notebook content storage

Delivered 2026-10-09 on branch `notebook`.

- Contract package `internal/notebookcontract`: bounds (256KiB body / 16KiB
  comment / 100-row pages), trusted `ScopeID` (`home`, `ws.v1:<id>`), actor +
  origin rules (workflow actors cannot spoof `generated` on ordinary writes),
  13 spec error codes carried by `*notebookcontract.Error`, canonical
  request digests, all §7 DTOs, `Store` + reserved `GeneratedWriter` ports.
- Migration 036 `notebook_content` (paired sqlite/postgres): sections,
  entries, immutable revisions, comments + comment_versions, mutation
  receipts. Seeds the 4 system-role sections per home scope; imports legacy
  `notes` rows once into `home/section-notes` as `origin='legacy'` with
  `legacy:` revision IDs.
- `internal/storage/notebook.go` shared helpers (IDs, cursor codec bound to
  scope+filter, envelope/bounds validation); `Notebook()` added to the Engine
  contract; per-backend `notebook.go` implementing all 16 §7 ops inside the
  existing transaction helpers: receipt lookup before state checks, one
  transaction per mutation, version+base_revision CAS, conflict re-probe so
  concurrent identical retries converge on one receipt.
- Read-only openers (`sqlite.OpenExisting`, `postgres.OpenExisting`) plus
  `storage.ExportNotebookEntry` and `vivy notebook export` — no App/runtime/
  module factory, no hidden migration; pre-036 deployments report an upgrade
  requirement.
