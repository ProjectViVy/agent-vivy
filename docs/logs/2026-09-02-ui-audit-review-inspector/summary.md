# UI-AUDIT-REVIEW-INSPECTOR: Run Inspector Review tab via shared ReviewCard

## Scope

`docs/architecture/hitl-review-center.md` §UI 要求 "The run inspector uses the
same `renderReviewCard` renderer for inline decisions"，但 Run Inspector 只有
run/background/children 三个 tab，审批只能去 Review Center。

- 新增 `ui/src/components/approvals/ReviewCard.tsx`：从 ApprovalsView 详情抽出的
  共享审批卡（标题 + 状态徽章 + 审计 dl + prompt/preview(差异优先)/risk/脱敏参数
  + pending 时内联决策控件）。ApprovalsView 详情与 RunInspector 复用同一组件，
  满足 "same renderer" 约束。
- `RunInspector.tsx`：Tabs 改受控并加第 4 个「审批」tab（计数），按当前 run 的
  `run_id` 过滤 store reviews；切到该 tab 时刷新 `loadReviews()`（事件驱动的
  approval/question 刷新保持不变）；卡片动作直接走 `respondReview`，与队列同一
  忙碌锁（reviewBusyIds）。
- ApprovalsView 改为薄壳：列表 + 布局保留，详情体替换为 `ReviewCard`（`key` 按
  review id 重挂载，保持"换选中清空草稿"的旧行为）。`statusLabel` 改由 ReviewCard
  导出复用。
- i18n en/zh 增 `runInspector.review`（审批 {{count}}）与 `runInspector.noReviews`。

## Explicitly not done

- 未为"运行中真实产生审批 → inspector 内联决策"写 e2e：需要真实 provider 触发
  工具审批；spec 只覆盖入口可达 + 空态（provider 无关）。
- 未改 reviews 的轮询策略（无新轮询；仍为打开 run / 事件 / 切 tab 时刷新）。
- 未动 Review Center 队列的交互与 Review sheet。
