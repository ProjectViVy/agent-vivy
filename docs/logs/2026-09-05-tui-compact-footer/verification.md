# Verification

## 自动化

- `go test ./sdk/tui/view/ -count=1`：通过（含 `TestFooterUsesShiftTabModeAndCtrlXShortcuts`）。
- `just ci`：通过。

## 真实路径 smoke

- 未读取或写入 `data/vivy.db`、`data/demo/`、`data/workspaces/`。
