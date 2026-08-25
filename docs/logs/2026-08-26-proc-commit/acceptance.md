# Acceptance — PROC-COMMIT 拆分入库

Date: 2026-08-26

## 如何确认收尾完成

1. `git log --oneline -5` 顶部为三个聚焦功能提交 + 本收尾 docs 提交：

   ```
   <hash> docs: close PROC-COMMIT after split commits
   1a3f0c7 feat(ui): add chat message action bar with copy and regenerate
   9a11370 feat(ui): add first-run welcome wizard
   a857976 feat(ui): add evolution page with demo governance workflow
   ```

2. `git show --stat a857976` / `9a11370` / `1a3f0c7`：每个提交只含各自主题的
   文件 + 自己的 `docs/logs/<主题>/`；没有任何一个提交同时含两个主题的核心文件
   （如 evolution 提交不含 `WelcomeWizard.tsx`，chat 提交不含 `EvolutionView.tsx`）。

3. 共享文件拆开可见：
   - `git show a857976 -- ui/src/i18n/zh.ts`：只有 nav/skills.status/evolution/demo.evolution
     词条；`git show 1a3f0c7 -- ui/src/i18n/zh.ts`：只有 `chat.copy` 等词条；
     `git show 9a11370 -- ui/src/i18n/zh.ts`：只有 `welcome.*` 词条。
   - `git show 9a11370 -- ui/e2e/runtime.spec.ts` 与 `git show 1a3f0c7 -- ui/e2e/runtime.spec.ts`
     分别只含向导适配与功能栏断言的 hunk。

4. `git status` 干净（根树回到零未提交改动的单 lane 状态）。

5. `docs/TODO.md`：§0.1 不再有 PROC-COMMIT 行（UI-EVO、UI-CHAT-ACT 保留为
   后续真实后端接线的 OPEN 项）；§10 Completion log 顶部新增 2026-08-26
   PROC-COMMIT 完成条目。

6. 未推送：`git status` 的 ahead 计数为本地领先远端的提交数，push 待用户授权。
