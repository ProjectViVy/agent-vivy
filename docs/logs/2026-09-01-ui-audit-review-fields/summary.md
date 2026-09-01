# Summary

## 主题

UI-AUDIT-REVIEW-FIELDS：审批中心详情补齐审计字段，消除「过期/失效原因不可审计」缺口。

## 背景与审查发现

2026-08-31 UI 审计确认：后端 `ReviewItem`（`ui/src/lib/api.ts`）已携带
`actor`、`created_at`、`expires_at`、`decided_at`、`precondition_hash`、
`stale_reason`、`decision_reason`、`error` 等审计字段，但
`ApprovalsView.tsx` 详情 `<dl>` 只渲染
run/action/target/effect/reversibility/scope/trust——过期审批为何失效、
拒绝理由是什么、审批者是谁均无处可看。

## 改动

- `ui/src/components/approvals/ApprovalsView.tsx`：详情 `<dl>` 在
  trust 之后追加（全部按字段存在性条件渲染）：
  - `actor`（发起者）
  - `createdAt` / `expiresAt`（必显，`new Date(x).toLocaleString(dateTimeLocale())`
    —— 与 PersonaMemoryView/CronTaskManagementView 同一 i18n 感知格式）
  - `decidedAt`（决定时间，有 decided_at 才显示）
  - `precondition_hash`（`<code>` + break-all，哈希原样）
  - `staleReason` / `decisionReason`（失效/决定理由）
  - `error`（text-destructive 标红）
- `ui/src/i18n/en.ts` + `zh.ts`：`approvals` 块 `trust:` 锚点后新增
  8 键（createdAt/expiresAt/decidedAt/actor/precondition/staleReason/
  decisionReason/errorLabel），双语同步。

## 明确不做

- 列表（master 侧）行不加字段——详情页才是审计面，列表保持可扫读。
- 不改 pending 操作区/流程逻辑；本行只补只读审计展示。
- UI-AUDIT-REVIEW-INSPECTOR（Inspector inline review）与
  UI-AUDIT-REVIEW-BUSY-SCOPE（按 item 锁）是另外的行，不混入。
