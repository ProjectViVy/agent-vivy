# Summary: UI audit cleanup (maintainer instruction list)

Follow-up to the 2026-08-31 audit (`docs/logs/2026-09-01-remove-chat-preflight`
uncovered a wider pattern). The maintainer reviewed the audit findings and gave
a numbered ruling; this iteration implements it.

## Done

1. **Remove orphaned `ScanPrompt` (kernel).** The prompt-injection scanner in
   `internal/tools/security.go` lost its only consumer when the preflight was
   removed in `86c94fa`. Deleted `ScanPrompt`, `SafetyFinding`, the
   `promptInjectionPattern` regex, and its test. `ValidateArgsSafety` and
   `RedactSensitive` stay (live callers in tooladapter/broker/worker). A
   `vivy dry-run` command was explicitly rejected by the maintainer.
2. **Dashboard: remove the 会话 (overview) tab.** `DashboardDemoView` now has
   exactly two tabs — Token 统计 and 轨迹 (both kept per maintainer ruling).
   Deleted the fake session/run/review numbers and the recent-activity feed,
   plus `getDemoDashboard`, `DemoDashboardSnapshot`, the
   `vivy.demo.dashboard` storage key, and their tests/i18n.
3. **Skills: remove the 变更请求 (change requests) tab.** The staged-revision
   review surface in `SkillsView` is gone along with `listSkillRevisions`,
   `SkillRevision`, and their i18n. The backend `skills/revisions/list` RPC and
   the SkillRevisions store stay untouched — they are written by the runtime's
   staged skill-write path, not by this UI.
4. **Chat input: relocate 新建会话.** The bottom-row Plus button (previously a
   「更多」stub that only raised "not wired up") is now the real 新建会话
   button; the duplicate 新建会话 in the upper toolbar row was deleted, as was
   the 更多 stub (`chatInput.more`/`moreUnavailable` i18n removed).

## Explicitly not done (maintainer ruling)

- 记忆 / 记事本 / 人格 / 进化 demo pages **stay** as-is with their demo data;
  they are slated for real backend integration later.
- Remaining hookless buttons (附件, AutoDream, 思考模式, 智能体模式 ask-branch,
  消息「编辑/回到这里/从此分叉」) stay for now.
- Backend `skills/revisions/list` method and store unchanged.
