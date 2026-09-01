# Summary

## 主题

UI-AUDIT-REVIEW-BUSY-SCOPE：Review 响应的忙碌锁从「全队列」改为「按 item」。

## 背景与审查发现

2026-08-31 UI 审计确认：`reviewBusyId: string | null` 非空时
ApprovalsView 禁用所有列表行、刷新与全部动作按钮——响应一条审批期间
用户连查看其他记录都做不到。store 的 `respondReview` 还以全局 early-return
串行化一切响应。

## 改动

- `ui/src/lib/store.ts`：`reviewBusyId: string | null` →
  `reviewBusyIds: string[]`；`respondReview` 按 id 单飞（同一 id 重复
  响应仍 early-return），不同 id 可并发；finally 按 id 移除。
  乐观状态映射与 `loadReviews()` 均按 id/全量幂等，并发安全。
- `ui/src/components/approvals/ApprovalsView.tsx`：
  - 列表行 `disabled={busyIds.includes(review.id)}`——一条响应中，
    其余行可浏览/选中；
  - 详情动作（textarea + 批准/拒绝/回答/取消）按
    `selectedBusy = busyIds.includes(selected.id)` 锁定；
  - 刷新按钮仅按 `phase` 门控（读操作，随时可刷）。
- `ui/src/routes/_layout.tsx`：Review sheet 关闭守卫
  （closeDisabled/Escape/点外/关闭拦截）改 `reviewBusyIds.length > 0`，
  语义不变：任何响应在途时 sheet 不可关闭。

## 明确不做

- 不改后端 `review/respond` 语义；并发上限由 UI 天然交互限制（人手速）。
- `reviewEpoch` 的 loadReviews 去抖保持原样——并发响应各自的
  `loadReviews()` 是幂等 GET，最后一次落库即真相。
