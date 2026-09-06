# 验证记录

命令均在 worktree `agent-vivy-tui-chat-body`（分支 `feat/tui-chat-body-polish`）执行：

- `go test ./sdk/tui/... -race -count=1` — 全绿（command/face/live/stream/view 均 ok）。builder 交付后与 reviewer P2 修复后各跑一次。
- `just ci` — CI-EXIT:0。kernel `go test ./...`、UI 测试、`cmd/vivy`/`cmd/vivy-code`/`ui` headless 编译、plugins（dingtalk/discord/feishu/lsp/qq/telegram）与 faces（headless/tui）plugin-ci 全部 ok。

新增测试（`sdk/tui/view/chat_body_test.go`）覆盖：ctrl+o 默认 8 行截断→展开→还原、`tui.debug` 与 ctrl+o 组合、ctrl+r reasoning 折叠→还原（同宽度 mdCache 失效）、空会话 hero 渲染与非空会话不渲染、gate 存在时两键不改变聊天体状态。
