# Verification — PROC-COMMIT 拆分入库

Date: 2026-08-26

## Gate: `just ci`（拆分前，仓库根，对整棵合并树）

EXIT=0：

- Go fmt-check / vet / test / headless-compile 通过
- UI typecheck 通过；单测 **57 passed（12 文件）**（含 `demo-api.evolution.test.ts`
  6 例、`use-welcome.test.ts` 5 例、`chat-actions.test.ts` 6 例）
- `vite build` ✓（仅既存 chunk-size 提示）

拆分操作本身不改动任何文件内容（只动 git index 与 TODO.md 登记行），拆分后
HEAD 树 == 拆分前工作树，故上述结果即最终提交态的门禁结果。

三个交付各自的 `just ci` / `just ui-e2e` / 3015 冒烟记录见：
`docs/logs/2026-08-25-evolution-page/verification.md`、
`docs/logs/2026-08-25-welcome-wizard/verification.md`、
`docs/logs/2026-08-26-chat-message-actions/verification.md`。

## 拆分正确性核查

- hunk 归属核对：`i18n/zh.ts`、`en.ts` 6 hunk 与 `runtime.spec.ts` 3 hunk 逐个
  确认单一主题（无混合 hunk）；en 与 zh hunk 结构一一对应。
- 跨主题污染 grep 为 0：`demo-api.ts`/`types.ts` diff 无 welcome/chat 词条；
  `ChatView.tsx`/`MessageBubble.tsx` 无 evolution/welcome 引用；
  `ConversationSidebar.tsx`/`SettingsView.tsx`/`SkillsView.tsx` 无跨主题改动。
- 每次提交前 `git diff --cached` 核对 staged 内容只含该主题（hunk 数、stat）。
- 拆分完成后 `git status` 仅剩 `docs/TODO.md`（PROC-COMMIT 收尾改动），
  即三个功能提交的合并内容与拆分前工作树逐字节一致。

## 跳过说明

- 本操作无用户可见行为变化、无代码改动，不适用浏览器冒烟
  （smoke 已由三个交付各自的 verification.md 覆盖）。
- 中间提交态未逐一跑 CI（见 summary.md「Explicitly not done」）。
