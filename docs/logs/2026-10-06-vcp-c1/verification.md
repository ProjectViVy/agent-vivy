# VCP C1 verification

## Focused tests

`go test ./internal/storage ./internal/runtime ./internal/rpc -run 'Tree|Clone|Import|Export'` — green.

- `TestCloneSessionCopiesVisibleView` — rewind folds msg-3/4, clone carries only the 2-row effective view; fresh ids; `session.cloned_from` event journaled with `parent_session_id`.
- `TestCloneEmptySession` — empty source clones to an empty child, no markers.
- `TestSessionTreeForkRewindForkEdges` — fork at msg-2, rewind, clone at effective tail; `session/tree` returns both edges kind=fork, correct `fork_point_message_id` per node, root has no parent.
- `TestImportSessionPiJSONL` — `testdata/pi-session.jsonl` (real pi v3 shapes: header, text/thinking/toolCall blocks, toolResult, model_change/usage/label entries, one malformed line): imported=5 rows, skipped=5; toolCallID/ToolArgs preserved; default title carries the source session id; `session.imported` event.
- `TestImportSessionMalformedHeader` — non-header first line and garbage both → `ErrImportMalformed`.
- `TestExportSessionHTML` — file written under `ExportDir`; visible view only (rewound row absent); CSP `default-src 'none'`; tool args escaped (`&lt;img src=x&gt;`); role counts footer; `format=pdf` rejected.
- `TestSessionTreeCloneImportExportRPC` — the four verbs round-trip through the control handler: clone → new session; tree → parent→child edge; import → 1 row; export → file on disk; param validation (empty data, missing session_id, bad format) → `InvalidParams`.

## Conformance

`ListSessionForkLinks` assertions added inside CN-21 (`session truncation markers`) — single-edge then edge+back-anchor ordering, both backends:

- `go test ./internal/storage/sqlite ./internal/storage/postgres` — green (sqlite conformance suite run).
- `go test ./sdk/internal/conformance/` — green; `internal/` source digest re-pinned to `2aae39a8…` (5 sites in `conformance_results.json`).

## Known limits

- `export_html`'s pi `outputPath` parameter is accepted but the kernel chooses the location (`<dataRoot>/exports/`) and returns the path — the sandboxed-writer contract; embedders read the path from the response.
- `get_entries` answers with the `session/messages` projection (`{entries:[{entryId,text}]}`), not pi's raw entry graph — sufficient for fork-message pickers, which is what embedders use it for.
- Thinking blocks from pi transcripts are dropped (counted in `skipped`) — Vivy rows have no replayable thinking surface.
- `just ci` deferred per-story (see plan index); conformance + focused suites green.
