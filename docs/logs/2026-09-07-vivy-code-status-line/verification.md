# Verification

All commands run from repo root, Windows / Git Bash, on the delivered tree
(content identical to commit `e2ad7f2`; the parallel window-title lane's
commits `2768390`/`83d242e`/`fc63529` are also present).

| Command | Result |
| --- | --- |
| `go build ./...` | OK |
| `go vet ./sdk/tui/...` | OK |
| `gofmt -l sdk/tui/` | clean |
| `go test -count=1 ./sdk/tui/view ./sdk/tui/live` | `ok … view 2.379s`、`ok … live 1.790s` |
| `go test -count=1 ./sdk/tui/...` (post-commit) | all `ok`（command/face/live/stream/view） |
| `just ci` | 全流程通过（kernel tests、plugins/{dingtalk,discord,feishu,lsp,qq,telegram}、faces/headless、faces/tui 均末段 `ok`，exit 0） |
| `go build -tags vivy_headless -o vivy-code.exe ./cmd/vivy-code` | OK（≈117 MB 二进制） |
| `TUI_PREVIEW=1 go test ./sdk/tui/view -run TestDumpComposerPreview` | 渲染三态确认（见下） |

## New deterministic tests

- `TestRenderInputChromeBusySpinnerElapsed` — busy chrome 含 braille 帧与 `run 1m30s`
- `TestRenderInputChromeQueuedCount` — `Queued=2` 显示 `queued 2`，为 0 时无 queued 字样
- `TestRenderInputChromeScrollIndicator` — 60 条溢出消息 + `scrollChat(-10)` 后右段含 `↓ ` 与 `end 回底`

## TUI_PREVIEW real-render dump（真实 View() 渲染路径，非 TTY 下 lipgloss 剥离淡色 ANSI）

```
chrome[busy+queued]: ⠙ run 1m15s  shift+tab 切换模式  shift+h 帮助  ctrl+x 快捷                    queued 2
chrome[scrolled]: shift+tab 切换模式  shift+h 帮助  ctrl+x 快捷                         ↓ 90% · end 回底
```

## Notes

- 交互式真终端冒烟无法在本环境自动化（无 TTY）：用 TUI_PREVIEW 全帧渲染 +
  `vivy-code.exe` 构建通过代替；人工步骤见 `acceptance.md`。
- `sdk/tui/live` 部分既有测试直接置 `l.busy`（不经过 `setBusyLocked`），
  产生零值 `BusySince` → chrome 显示 `⠋ run`（无计时），属规格内的
  零时间戳行为；生产路径 7 处切换已全部收敛到 helper。
