# Verification

## 门禁

- `just ci` — 通过（exit 0）：Go fmt/vet/test、headless 编译、plugin-ci
  6 module、UI install + `tsc --noEmit` + `vitest run` + `vite build`
  全绿（新增 8 个 approvals i18n 键 en/zh 对齐，详情 dl 扩展过类型检查）。
- `just ui-e2e` — 通过（exit 0，10 passed / 1 skipped，27.3s）：真实浏览器
  + 真实控制面；`runtime.spec.ts` 覆盖 review 主路径（打开 Review sheet），
  详情 dl 追加行未回归既有断言。

## Smoke 说明

- 审批详情新字段需要终态/过期记录才能全量观察（pending 记录只有
  created/expires 两行必显），无组件专属 spec 可造数；按 CH-C1-N3 先例
  以全套 e2e 为 smoke 替代，人工观察路径在 acceptance.md。
- WebSocket RPC 传输（`/rpc/bootstrap` → WS upgrade）无 curl smoke 路径。

## 复核证据（静态）

- `ui/src/lib/api.ts` `ReviewItem`：`actor?/created_at/expires_at/decided_at?/
  precondition_hash?/stale_reason?/decision_reason?/error?` 均为线格式
  已有字段，本改动纯前端渲染，无 RPC/后端变更。
- `ui/src/components/approvals/ApprovalsView.tsx`：dl 在 trust 后追加
  8 类行，全部按字段存在性条件渲染；时间统一
  `new Date(x).toLocaleString(dateTimeLocale())`（zh/en locale 感知，
  与 PersonaMemoryView/CronTaskManagementView 同一来源）。
- i18n：`approvals.{createdAt,expiresAt,decidedAt,actor,precondition,
  staleReason,decisionReason,errorLabel}` en/zh 同步新增，锚点 trust: 后。
