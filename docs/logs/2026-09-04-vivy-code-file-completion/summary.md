# VIVY CODE project file completion

## Changed

- Added a shared fullscreen `@file` completion popup for both the built-in and
  packed TUI faces. It debounces typing, queries the control plane, rejects
  stale request/query/session results, supports keyboard wrap navigation, and
  replaces only the active trailing token.
- Extended the shared input grammar with quoted and escaped file references.
  Server-returned paths containing spaces, quotes, Unicode whitespace, or
  Windows separators now round-trip through selection, retry restoration, and
  the existing `turn/start` context path contract.
- Extended `project-context/list` with a metadata-only `query` filter applied
  before the 200-result cap. Listing now probes at most 8 KiB per candidate,
  has a 4,000-entry walk budget, omits unsafe/binary/oversized paths, and never
  returns file bodies. Final send still re-resolves every path server-side.
- Sanitized untrusted completion, editor, attachment-chip, and file-chip text
  before terminal rendering. The shared view also rejects non-relative,
  traversal, volume/ADS, control, bidi-control, duplicate, and negative-size
  candidate metadata.

## Scope

This is a Vivy/kernel and VIVY CODE TUI change. Vivy Studio was not changed.
No tenant Journal data was read or written.

## Explicitly not done

- `/files` remains the separate run-workspace surface. Its pre-existing
  run/session ownership gap is recorded as `TUI-WORKSPACE-OWNERSHIP` in
  `docs/TODO.md`; it was not mixed into this project-context deliverable.
- Dynamic skill/MCP prompt completion and the remaining sidebar/model/diff
  parity items remain on their existing TODO tracks.
