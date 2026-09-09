# SET-RMW — Atomic read-modify-write for `settings.Update`

## What changed

Problem: the 8 settings write handlers in `internal/rpc` used cross-call
Load→modify→Save; package-level `fileMu` serialized only the file-I/O window of
each Load/Save, not the window between reading and writing back. When two
handlers interleaved, the later writer overwrote the earlier update with a stale
snapshot (last-writer-wins)—already identified as a remaining item when
SET-FILERACE fixed file corruption.

- `internal/app/settings`: extracts lock-free `load`/`write` internals (`Load`/`Save`
  retain their original semantics and the lock window is unchanged); adds
  `Update(path, fn)`, holding `fileMu` once across load → fn → validate → write.
  `fn` errors pass through unchanged (callers' domain errors cross the
  transaction); candidate-document validation failures wrap `*ValidationError`,
  with a new `IsValidationError` classification. `fn` must not call Load/Save/
  Update again (deadlock).
- `internal/rpc/control.go`: adds the `updateSettingsOrError` error-mapping
  helper and `settingsFnError` wrapper (domain `*Error` passes through
  `errors.As`). Migrates all 8 read-modify-write paths:
  `updateSettings`, `setActiveTools`, `upsertProvider`, `deleteProvider`,
  `refreshProviderModels`, `updateChannel`, `upsertMCP`, and `deleteMCP. The
  domain-level not-found checks move into `fn` (checking the fresh document
  instead of the old snapshot).
- `refreshProviderModels` becomes two-stage: stage one snapshots and resolves the
  target, then calls upstream `ModelLists.List` without holding the lock (a 15s
  network call no longer blocks other settings writes); stage two re-resolves by
  ID inside `Update`, also matching `(bundle, base_url)` for the first catalog
  clone so concurrent double-cloning cannot create duplicate rows (Validate
  rejects them). Merge logic is extracted into `unionModels` (same semantics:
  upstream order first, local additions retained).

## Behavior notes

- The single-user UI request/response shape is unchanged; the UI still echoes the
  persisted document.
- Error mapping changes: document I/O failures on write paths (read/temp-file/
  rename) were previously always mapped to `InvalidParams`; after classification
  they become `internal error`. Validation failures remain `InvalidParams`, with
  unchanged message text.

## Explicitly not done

- Cross-process locking: `fileMu` is an in-process lock, and the single-writer
  boundary for `settings.yaml` remains one process (consistent with the existing
  deployment model; no file lock was introduced).
- `config.yaml` is out of scope (read-only loading, with no write path).
- `Save` does not wrap `ValidationError` (only `Update` does; `Save`'s existing
  return-value semantics are unchanged, and `control.go` has no Save callers
  after the migration).
- No fsync/durability changes.
