# 2026-09-07 TUI sidebar Modified Files styling + terminal window title (D lane F10/F7 closure)

## What changed

Two VIVY CODE TUI deliverables from lane D's wrap-up list, implemented by
builder subagents under main-agent supervision:

### F10 — sidebar Modified Files: add/delete colors + middle-truncated paths

`sdk/tui/view/render.go`

- `+N` additions render with the diff-add green palette and `-N` deletions
  with the diff-red palette; path and timestamp stay dim.
- New `middleTruncate` helper (1/3 head, `…`, 2/3 tail) keeps the filename
  visible on long paths; built on pinned `x/ansi` Truncate/TruncateLeft with
  a measured-width retry so wide runes / grapheme clusters never overshoot.
- Rows are assembled per styled segment (ANSI survives sizing). The path
  keeps a minimum 10-cell budget (`minModifiedPathCells`); the timestamp is
  dropped before the path starves; counts always render. Degenerate tiny
  widths fall back to the legacy whole-line truncation.

Tests: new `sdk/tui/view/modified_files_test.go` (middle-truncate behavior,
CJK boundary safety, styled-count and fit assertions at production width 32).
`sdk/tui/view/view_test.go` 20-file scroll fixture drops `UpdatedAt` — with
the new budget a full-width timestamp would erase the filenames the scroll
assertions depend on.

### F7 — terminal window title

> Audit fix (commit `bb5e294`): the original command-channel deferral starved
> in production — the live driver's 40ms heartbeat keeps a command pending on
> every update. The sync now goes through the wired `tea.Program` handle and
> applies immediately; see `verification.md` for the full record.

`sdk/tui/view/model.go`

- `Init` applies the bare `VIVY CODE` brand immediately (batched with the
  driver's own init when present).
- `Update` keeps the title in sync with the active session title
  (`VIVY CODE · <title>`) only when it actually changes. Titles are
  sanitized (control chars collapsed, 64-rune cap) before reaching the
  terminal.
- `New` seeds the brand so no redundant title command fires before the first
  real sync (protects existing `cmd == nil` test assertions).
- Contract: while an update already carries driver commands the title sync
  defers to the next quiet update, so executing the returned command still
  yields the driver's own message instead of a batch wrapper. Pinned by
  `TestUpdateDefersTitleSyncWhileDriverCommandsArePending`.

Tests: new `sdk/tui/view/window_title_test.go`.

## Explicitly not done

- A third lane's in-progress chrome busy-timer work (BusySince plumbing in
  `surface.go`/`controller.go`, spinner chrome in `render.go`/`model.go`,
  `chrome_status_test.go`) was edited concurrently in the shared root tree.
  This iteration's commits exclude it via hunk-scoped staging; it belongs to
  its own deliverable and its own log.
- Pre-existing untracked `sdk/tui/view/zpreview_test.go` (env-gated preview
  dumper from earlier lane work) and scratch `tui-composer-shot.png` were
  left out of these commits on purpose.
- No push (requires explicit authorization).
