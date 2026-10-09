# N0 — Remove automatic notebook injection: summary

Date: 2026-10-09. Branch: `notebook`. Requirements: NB-01, NB-07.

## What changed

- Deleted the legacy preamble projection at its source: `ServiceDeps.Notes`,
  `Service.notesDigest`, `formatNotesDigest`, the digest bound constants and
  the `notesDigest` parameter of `composeRunPreamble`.
- Removed the App wiring `ServiceDeps{Notes: backend}` initialization and all
  compiled test call sites that supplied the deleted dependency.
- Updated prompt/context/soft-plan tests to the surviving signature and kept
  an explicit assertion that the preamble never opens a notebook section.
- Refreshed the checked-in internal source digest in
  `sdk/internal/assembly/conformance_results.json` (tree hash moved by the
  internal/ edits; regenerated with `go run ./sdk/internal/cmd/source-hash internal ""`).

## What was deliberately kept

- `internal/tools/notes.go`, `writenote.go`, the three note tools, the legacy
  `notes` table, all stored notes and all saved chat messages. Explicit note
  reads still work; N2 rebinds them to the scoped service.
- No flag, replacement ContextSource, cache or observer was added.

## Removed symbols

`ServiceDeps.Notes`, `Service.notesDigest`, `formatNotesDigest`,
`digestNoteLimit`, `digestLineLimit`, the `composeRunPreamble` digest
parameter. `rg 'notesDigest|formatNotesDigest|deps.Notes'` is empty in
production code.
