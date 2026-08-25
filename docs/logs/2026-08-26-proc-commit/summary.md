# PROC-COMMIT 收尾：根树三个已完成主题拆分入库

Date: 2026-08-26
Status: complete

## What changed

关闭 `docs/TODO.md` §0.1 的 PROC-COMMIT：根工作树里堆放的三个已完成但未提交的
交付，按 `commit-one-concern-per-deliverable` 拆分为三个聚焦提交，无代码改动。

| 主题 | 提交 | 内容 |
|---|---|---|
| evolution 页 | `a857976` `feat(ui): add evolution page with demo governance workflow` | `EvolutionView` / `useEvolution` / `demo-api`+`types` 域类型 / 路由与 `routeTree.gen.ts` / 侧栏入口 / `skills.status` 徽章 + 各自 `docs/logs/2026-08-25-evolution-page/` + §0.1 `UI-EVO` 登记 |
| welcome wizard | `9a11370` `feat(ui): add first-run welcome wizard` | `WelcomeWizard` / `use-welcome`(+测试) / `_layout` 挂载 / 设置页重跑入口 / `welcome-float` 样式 / e2e spec + `playwright.config.ts` zh-CN / `docs/logs/2026-08-25-welcome-wizard/` |
| chat message actions | `1a3f0c7` `feat(ui): add chat message action bar with copy and regenerate` | `chat-actions`(+测试) / `MessageBubble` 功能栏 / `ChatView` 接线 / e2e 断言 + `docs/logs/2026-08-26-chat-message-actions/` + §0.1 `UI-CHAT-ACT` 登记 |

## 共享文件拆分方式

交叉修改的共享文件按主题 hunk 分块 stage（`git diff` hunk 归属核对后
`git apply --cached` 子集 patch）：

- `ui/src/i18n/zh.ts` / `en.ts`：6 个 hunk——1/3/4/5（nav 词条删除、`skills.status`、
  `evolution` 域、`demo.evolution`）→ evolution；2（`chat.copy` 等）→ chat；6（`welcome.*`）
  → welcome。zh/en 同步 stage，中间提交保持词典结构一致。
- `ui/e2e/runtime.spec.ts`：3 个 hunk——1（向导跳过）、3（`vivy.ui.welcome.completed`
  白名单 + 「返回列表」→「返回」）→ welcome；2（功能栏断言 + 剪贴板）→ chat。
- `docs/TODO.md`：3 行新条目按行级手术分别随各自交付提交；PROC-COMMIT 行随本
  收尾提交关闭（§0.1 移除、§10 登记）。

提交前核对单主题文件无跨主题污染（`demo-api`/`types` 无 welcome/chat 改动、
`ChatView`/`MessageBubble` 无 evolution/welcome 改动等，grep 全部为 0）。

## Explicitly not done

- 未推送（push 需用户明确授权）。
- 未对三个中间提交态逐一跑 `just ci`（拆分只动 index，工作树始终为合并态；
  最终态整树复跑 `just ci` 全绿，且各中间态 zh/en 词典结构同步、无悬空引用）。
- 未改动任何产品代码；三个交付自身的验证记录见各自 `docs/logs/*/verification.md`。
