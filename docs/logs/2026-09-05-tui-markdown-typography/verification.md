# Verification

## 自动化

- `go test ./sdk/tui/view/ -count=1`：通过（含 `TestRenderMessageMarkdownTypography`、user/reasoning、窄窗口/bidi、finished-message cache）。
- `just ci`：通过（fmt-check、UI typecheck、24 个 Vitest 文件 / 201 项测试、UI build、Go vet、全仓 Go test、headless compile、全部 plugin/face 独立模块；`sdk/tui/view` 与 `faces/tui` 均绿）。

## 真实路径 smoke

- 排版契约由 view 测试钉死（glamour `WithStyles` + TrueColor，不依赖终端探测）：H1 去 `#`、H2 保留 `##`、列表 `•`、引用 `│`、围栏代码保留 `fmt.Println`、思考块 QuietMarkdown + `┊`。
- 未读取或写入 `data/vivy.db`、`data/demo/`、`data/workspaces/`。

