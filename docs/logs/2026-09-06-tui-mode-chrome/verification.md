# Verification

## 自动化

- `go test ./sdk/tui/view ./sdk/tui/live`：通过。
- `just ci`：通过。

## 真实路径 smoke

- 未读取或写入 `data/vivy.db`、`data/demo/`、`data/workspaces/`。
