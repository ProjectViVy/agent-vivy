# Verification — TUI-MD-STREAM-CACHE

| Command | Result |
|---|---|
| `gofmt -l sdk/tui/view` | clean (no output) |
| `go build ./sdk/tui/...` | ok |
| `go test ./sdk/tui/view ./sdk/tui/live` | `ok agent-vivy/sdk/tui/view`, `ok agent-vivy/sdk/tui/live` |
| `just ci` (repository root, background run) | completed, exit code 0 |

## 真机说明

本次切片为渲染缓存策略移植：流式气泡的可见内容与完成态渲染路径与此前
完全一致（单测钉死同一 renderMessage 渲染契约），不改变任何键位/布局。
与 `docs/logs/2026-09-07-tui-chat-body-polish/verification.md` 同口径的
Windows Terminal 真机评审建议在人类验收时执行：长答案流式输出期间观察
渲染逐 tick 更新、无块间撕裂；本目录 `acceptance.md` 列出验收点。
