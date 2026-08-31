# VC-1f — UI diff 渲染（unified/split + 统计 + Review Center）

日期：2026-08-31 · 分支：`feat/vc1a-bash-tool`（worktree `agent-vivy-vc0`）

## What changed

服务端一半（go-udiff，标准统一 diff）与 UI 一半（自写渲染器）同属本交付：

- **服务端**：`internal/runtime/filesystem_backend.go` 的 `boundedDiff` 改用
  `go-udiff`（`github.com/aymanbagabas/go-udiff`，MIT，自研清单第 2 项）输出
  标准统一 diff（3 行上下文、`--- a/…`/`+++ b/…` 头、`@@ -l,c +l,c @@` hunk），
  保留 32 KiB `maxDiffBytes` 截断帽（截断后追加 `[diff truncated]` 标记行）。
  5 个调用点（WriteFile / PatchFile / MultiPatchFile / PrepareWriteFile /
  PreparePatchFile / PrepareMultiPatchFile）随之自动升级；旧的单 hunk 伪 diff
  生成器删除。`go.mod` 中 go-udiff 由间接依赖提升为直接依赖。
  两个 Go 测试的 diff 断言同步更新（真实统一 diff 上下文行带 ' ' 前缀）。
- **UI 解析器**：`ui/src/lib/diff.ts` —— 宽松统一 diff 解析：不信任 hunk 头
  声明的行数（截断会砍断尾部 hunk），只按行类型推进行号；容忍伪 diff（裸
  `@@`、上下文无前缀）、`\ No newline`、`[diff truncated]` 标记；文本缺文件
  头或缺 hunk 时返回 null（调用方回退纯文本）。附 `diffSplitRows`（删除块/
  新增块按下标配对供分栏渲染）与 `parseToolResultDiff`（从 FileMutationResult
  JSON 提取 diff/path）。
- **UI 组件**：`ui/src/components/ui/DiffView.tsx` —— 对照 Crush 呈现行为
  （D10 拍板，FSL-1.1-MIT 下行为对齐、零代码复制，渲染器自写）：
  `+N −M` 统计、unified/split 双模式切换、行号 + 增删着色（emerald/rose）、
  hunk 头行、截断提示；`max-h-96` 滚动容器；解析失败时保底按纯文本 `<pre>`。
- **接入点 1（Review Center）**：`ApprovalsView.tsx` 审批详情 `preview` ——
  `looksLikeDiff()` 判定为 diff 时用 DiffView，否则维持原纯文本 `<pre>`
  （bash 等非 diff 审批不受影响）。顺带关闭 FACE-TUI-2「审批 diff 高亮」余项。
- **接入点 2（聊天气泡）**：`MessageBubble.tsx` 工具结果分支 —— 内容为
  FileMutationResult JSON（含非空 `diff` 字段）时渲染「工具结果 + 路径 +
  DiffView + 折叠原始 JSON」；其余工具结果（bash/grep/read 等）维持原样。
- **i18n**：新增 `diff.{unified,split,truncated,statsLabel}` 与
  `chat.toolResultRaw`，zh/en 同步（词典 parity 测试覆盖）。
- **测试**：`ui/src/lib/diff.test.ts`（解析/配对/截断/工具结果提取 12 例）、
  `ui/src/components/ui/DiffView.test.tsx`（SSR 渲染冒烟 2 例）。

## Explicitly not done

- LSP/诊断 diff 着色与语法高亮（Crush 用 tree-sitter 高亮，属后续拍板范围）。
- `filetracker`/`file_versions` 陈旧读保护（RB-1，等 O1..O6 拍板，未动）。
- 既往历史数据里的旧伪 diff（无 `--- a/` 头的裸 hunk 变体）也能被宽松解析器
  渲染，但未做迁移——旧消息保持原样可读。
- MessageBubble 编辑 / 回退 / 分叉仍为占位（不属于本卡片）。
