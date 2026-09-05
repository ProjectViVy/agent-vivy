# TUI composer chips — verification

Date: 2026-09-06

## Commands

| Check | Result |
|---|---|
| `gofmt` on `sdk/tui/view/{styles,layout,render,model,view_test,composer_test}.go` | applied |
| `go test ./sdk/tui/view/` | **pass** |
| `go test ./sdk/tui/...` | **pass** |
| `just ci` | **pass** (fmt-check, ui-ci, vet, test, headless-compile, plugin-ci) |

No browser smoke: TTY-only deliverable. Composer chrome is covered by
`View()` / `renderEditor` string assertions under fixed widths, not a
live console session.

## Notes

`TestChatViewportPreservesHistoryAndFollowState` now asserts that page-up
hides the latest history line instead of pinning `history-30`, because
the taller composer shrinks the chat viewport by one row.
