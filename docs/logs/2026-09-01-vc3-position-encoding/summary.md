# VC-3 slice 6: LSP positionEncoding negotiation (+ diagnostics-wait race fix)

Date: 2026-09-01. Lane: `feat/vc1a-bash-tool` (worktree `agent-vivy-vc0`).

## What changed

Two fixes in `plugins/lsp`, both about position correctness:

- **positionEncoding negotiation (LSP 3.17)** — previously the plugin
  assumed UTF-16 code units unconditionally. The initialize handshake now
  sends `general.positionEncodings: ["utf-16","utf-8","utf-32"]` and reads
  back the server-chosen `capabilities.positionEncoding` (absent = the LSP
  default utf-16; an unknown unit also falls back to utf-16). The
  negotiated unit is stored on the connection and drives the byte-offset
  math: `utf16Offset` became `offsetAt(content, pos, enc)` counting
  utf-16 units / utf-8 bytes / utf-32 code points, and `lsp_rename`'s
  `applyEdits` interprets WorkspaceEdit ranges in the negotiated unit.
  Servers that pick utf-8 (e.g. configurable in gopls) no longer corrupt
  edits around non-BMP characters — a naive utf-16 count on a utf-8
  position slices emoji into invalid bytes, and there is now a test
  asserting the encoding actually flows through.
- **Diagnostics-wait race fix** — `waitForDiagnostics` snapshotted the
  per-URI publish generation *after* the didOpen/didChange was sent, so a
  publish that landed before the snapshot made the wait loop wait 3s for
  a fresh round that already happened (`lsp_diagnostics` intermittently
  returned the "wait_ms elapsed" tombstone; `TestDiagnosticsToolEndToEnd`
  was flaky under load). Callers now snapshot `diagGeneration(uri)`
  strictly before the sync request and pass the base in. Both
  `lsp_diagnostics` and the VC-3 backfill observer use the corrected
  ordering.

## Boundaries (deliberate)

- Tool-facing columns stay in the negotiated server unit (as reported and
  as accepted) — the lsp_* family round-trips its own positions. The
  model-facing rune-vs-units mismatch for non-ASCII lines (columns
  sourced from `read_file`'s rune numbering) is pre-existing and shared
  with the utf-16 default; it is not a negotiation concern.
- utf-16 remains the client's first preference, so well-behaved servers
  keep today's behavior byte-for-byte.

## Crush alignment

Behavior/protocol alignment only; zero code copied (Crush is FSL-1.1-MIT).
LSP position negotiation is protocol-standard behavior, not a Crush
feature port.

## Explicitly not done

- Model-facing column unit unification (rune columns everywhere) — a
  separate contract change if wanted.
