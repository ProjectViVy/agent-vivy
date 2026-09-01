# UI-AUDIT-REVIEW-NAV：/approvals 主导航入口

## What changed

- `ConversationSidebar` 主导航 NAV_ITEMS 在 Dashboard 与 Cron 之间新增 `/approvals` 项（ShieldCheck 图标，与 ApprovalsView 标题同款）——审批中心完整路由此前只能从聊天 shield 打开 sheet，跨会话 Review Center 全页面不可发现（2026-08-31 审查结论）。
- i18n：`nav.approvals` en=`Approvals` / zh=`审批中心`，与既有 `layout.reviewCenter` / `approvals.title` 文案一致，无新语义。
- 新增 e2e `ui/e2e/approvals-nav.spec.ts`：不依赖 provider——主导航点击 → URL `/approvals` + 标题/副标题断言 + 无 demo localStorage + 移动断点（390px）抽屉内入口可达；钉死「主导航可达」不再回退。

## What was explicitly NOT done

- Run Inspector 的 Review inline/tab（UI-AUDIT-REVIEW-INSPECTOR）另单。
- 不改 ApprovalsView 本体与 review sheet 逻辑；导航项无 pending 徽标（现有 RPC 无轻量计数字段，不为此扩面）。
