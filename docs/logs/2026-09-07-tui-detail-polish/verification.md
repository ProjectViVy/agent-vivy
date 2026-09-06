# 验证记录

环境：Windows，worktree `C:\Users\Administrator\Desktop\morediva\diva-go\agent-vivy-tui-polish`，分支 `feat/tui-detail-polish`。

## 逐批次门禁（每特性提交后由实现子代理跑，主代理复核）

- 批次 A 后（`73aa834`..`02781d8`）：`go build ./sdk/... ./cmd/...` 绿；`go test -count=1 ./sdk/tui/...` 5 包全绿（view 0.739s）；`gofmt -l sdk/` 无输出；`go vet ./sdk/tui/...` 通过。
- 批次 B 后（`8dbf8ff`..`a345a51`）：同上全绿（view 0.843s）；`gofmt -l sdk/` 无输出。

## 主代理独立复核（收尾）

```
go test -count=1 ./sdk/tui/...
ok  agent-vivy/sdk/tui/command   ok  agent-vivy/sdk/tui/face
ok  agent-vivy/sdk/tui/live      ok  agent-vivy/sdk/tui/stream
ok  agent-vivy/sdk/tui/view
gofmt -l sdk/  → 无输出（FMT-OK）
git status → 干净（仅未跟踪 sdk/tui/view/zpreview_test.go 视觉辅助）
```

## 视觉自查（TUI_PREVIEW）

`TUI_PREVIEW=1 go test ./sdk/tui/view -run TestDumpComposerPreview -v`，批次 A 与 B 各特性实现后手检整帧输出：

- 多行输入各行可见、光标在末行尾；超 6 行出现 `…` 上截。
- 空输入显示暗色占位符；输入后消失。
- busy 帧（`⠴ 1m12s`）与 queued/host/标题右段（`⏸ 2 queued  127.0.0.1:8787`）无乱版。
- 悬停历史时出现 `↑ 历史 · 下方还有 30 行`；回底后消失。
- 无 ANSI 破版、无宽度溢出。

## 新增测试覆盖（摘要）

- F2：`editorInputLines` 分行/截断/尾窗/`…` 标记/CJK 宽字符（lipgloss.Width）/ANSI 行；`editorReserve` 增长封顶；`mainH` 递减、下限 1；gate 单行预留。
- F3：占位符四态（空/有草稿/gate/侧栏聚焦）。
- F4：2000/2001 字符与 40/41 行边界；chip 与附件 chips 共存顺序；预留随 guard 增减。
- F11：busy/idle 边框前景断言（`GetBorderTopForeground()`；pin 的 lipgloss 无 `GetBorderForeground()`，见 summary 偏差说明）。
- F1：帧取模、busy→idle 复位、idle 不续排、`<1s`/`12s`/`1m05s` 文案表、err 优先。
- F12：`joinChromeRow` 整宽/CJK/降级次序（标题→host→queued）、queued=0、超宽左段截断。
- F6：两态提示决策表、`G` 空输入回底/非空进草稿/busy 守卫、近底隐藏、离底自动 follow=false（核对既有 `scrollChat` 已覆盖）。

## just ci（收尾全量）

`just ci`（fmt-check → ui-ci → vet → test → headless-compile → plugin-ci）：**exit 0，全绿**。

- fmt-check / vet ./...：通过。
- ui-ci（pnpm install --frozen-lockfile + typecheck + test + build）：通过。
- `go test ./...`：全绿，其中 `sdk/tui/view 5.354s`（覆盖本提案全部新测试）、
  `internal/runtime 165.6s`、`internal/rpc 70.3s` 等；无 flake 触发。
- headless-compile（`-tags vivy_headless ./cmd/vivy ./cmd/vivy-code ./ui`）：通过。
- plugin-ci：plugins/dingtalk/discord/feishu/lsp/qq/telegram + faces/headless/tui 全绿。

工作树提交后状态：干净（仅未跟踪视觉辅助 `sdk/tui/view/zpreview_test.go`，不提交）。
