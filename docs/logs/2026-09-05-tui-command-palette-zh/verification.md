# Verification

## 自动化

- `go test ./sdk/tui/... -count=1`：通过。
- `just ci`：通过（fmt-check、UI、vet、全仓 Go test、headless compile、plugin/face 模块；`sdk/tui/view` 与 `faces/tui` 均绿）。

## 真实路径 smoke

- 排版与中文契约由 `TestCommandPaletteHighlightsMatchesAndHidesUnselectedUsage` 等用例钉死。
- 未读取或写入 `data/vivy.db`、`data/demo/`、`data/workspaces/`。
