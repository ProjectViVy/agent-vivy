# TUI 细节打磨（批次 A + B）— lane 交付摘要

- 分支：`feat/tui-detail-polish`（worktree `agent-vivy-tui-polish`，基于 main@c64b63a + 提案 `92c33ae`）
- 提案：`docs/plans/2026-09-06-tui-detail-polish.md`（12 特性、4 批次）
- 日期：2026-09-07

## 本 lane 交付（8 提交，每特性一个）

| 特性 | 主题 | Commit |
|---|---|---|
| F2 | composer 多行输入完整可见 + 自适应高度（`editorInputLines` 纯函数、`maxEditorLines=6` 尾窗 + `…` 上截标记、`editorReserve` 扩参） | `73aa834` |
| F3 | 空输入暗色占位符（`composerPlaceholder`，gate/草稿/侧栏聚焦不显示） | `7f60072` |
| F4 | 大粘贴警示 chip（`pasteGuardChip`，阈值 2000 字符 / 40 行，随草稿实时消长） | `b93af3e` |
| F11 | busy 态 composer 边框降亮（`p.Dim`，busy 优先于工作模式色） | `02781d8` |
| F1 | braille spinner + 本地观察计时（`spinnerTickMsg`/`tea.Tick(120ms)` 续排、`spinnerLabel` now 注入、meta.Error 优先） | `8dbf8ff` |
| F12 | chrome 行右段环境元信息（`joinChromeRow` 整宽拼装；queued/host/标题按 标题→host→queued 降级） | `15c7950` |
| F6 | 滚动悬停提示 + `G` 回底（`chatScrollInfo` 渲染期传递、距底 ≤3 行隐藏） | `a345a51` |
| docs | 提案回填 hash + 交付情况 + 本日志 + TODO 登记 | （本提交） |

## 执行期间的外部变化（重要）

另一 lane 在 main 上并行交付了同一提案的其余部分：F1/F12（`e2ad7f2`）、
F5/F9/F13（`9052ca5`，merge `b108f0c`）、F10（`2768390`）、F7（`83d242e` +
审计修复 `bb5e294`）。因此：

- **批次 C/D 未在本 lane 重复派单**（避免平行重复实现）。
- 本 lane 的 F1/F12 与 main 互为**平行实现**，保留在本分支（提案允许不合
  并）；若日后落地合并，建议取 main 版本，仅移植本分支独有的 F6 与批次 A
  （main 无对应实现）。
- 根工作树在本 lane 执行期间自始至终未被本 lane 触碰。

## 明确未做

- F8（按消息级 token/cost）：需 surface 协议新增，超出"纯显示细节"边界，已登记 `docs/TODO.md` §0.1（`TUI-DETAIL-F8`）。
- F12 compact 模式下 chrome 右段与头部 title/host 的信息重复（外观取舍），已登记 `TUI-DETAIL-CHROME-DUP`。
- 不改 surface 协议、不加依赖、不做鼠标支持、不引入 bubbles。
