# 验证记录

命令均在 worktree `agent-vivy-tui-chat-body`（分支 `feat/tui-chat-body-polish`）执行：

- `go test ./sdk/tui/... -race -count=1` — 全绿（command/face/live/stream/view 均 ok）。builder 交付后与 reviewer P2 修复后各跑一次。
- `just ci` — CI-EXIT:0。kernel `go test ./...`、UI 测试、`cmd/vivy`/`cmd/vivy-code`/`ui` headless 编译、plugins（dingtalk/discord/feishu/lsp/qq/telegram）与 faces（headless/tui）plugin-ci 全部 ok。

新增测试（`sdk/tui/view/chat_body_test.go`）覆盖：ctrl+o 默认 8 行截断→展开→还原、`tui.debug` 与 ctrl+o 组合、ctrl+r reasoning 折叠→还原（同宽度 mdCache 失效）、空会话 hero 渲染与非空会话不渲染、gate 存在时两键不改变聊天体状态。

## 真机评审补充（2026-09-07，Windows Terminal + vivy-code.exe）

以真实 `vivy-code.exe`（私有实例 Journal）跑通后逐项确认：

- **F13**：空会话 hero 渲染正确（wordmark、寻找真心之旅、cwd、命令/键位提示），UIA 文本与截图双证。
- **F9**：ctrl+r 把 reasoning 块折叠为单行 `┊ reasoning · 5 行 · ctrl+r 展开`，再按还原，双向验证（埋点日志 + 终端缓冲双证）。
- **F5**：真实 list_dir 回合中工具卡截断为 8 行正文 + `… 33 more lines · ctrl+o expand`（实例日志 `tool-cap-hit, omitted=33` 逐帧确认 `debug=false` 路径），ctrl+o 展开方向由单测钉死并经人类验收确认。
- 评审期间排查结论：曾出现的「截断/折叠未生效」为桌面焦点拉锯导致的按键落点与双次切换问题，非功能缺陷；代码、二进制（marker 串在包内）、`config.yaml`/`data/settings.yaml`（均无 `tui.debug`）三层核过，view 层无绕过渲染路径。诊断用临时埋点已还原，交付后根树无残留改动。
