# TUI composer chips

Date: 2026-09-06
Status: complete

## Outcome

The VIVY CODE terminal editor is a rounded composer. Model, permission
preset, and thinking mode sit as small dim text in the top-left of that
box. The Crush `::: ` prompt stays on the input row.

## Delivered

- `sdk/tui/view/styles.go`: `EditorBox` palette role (rounded border,
  muted, not the dialog violet).
- `sdk/tui/view/render.go`: `renderEditor` wraps chips + optional
  attachment metadata + Crush prompt in that box. `renderComposerChips`
  reads the same driver fields as the sidebar.
- `sdk/tui/view/layout.go`: reserved editor height is 4 cells, 5 with a
  pending attachment.
- Tests in `sdk/tui/view/composer_test.go` for corners, chips, narrow
  widths, gate marker, attachments, and help-row visibility.

Packed `faces/tui` and built-in `vivy-code` share this view; no face
fork.

## Explicitly not done

- Web `ChatInput` / `MaskAndModelSwitcher` (reverted earlier; stays out)
- Wiring `agent`/`plan` run mode into TUI `turn/start`
- Clickable chips or a new picker
- Sidebar restyle; those rows remain Crush's detail rail
