# UI-CRON-P2（可行部分）— 定时一次（`at`）调度暴露进 cron 表单

## What changed

cron 后端完整支持 `at` 单次调度（`ValidateCronSchedule` 只要求正 `atMs`，
`NextCronAfter`/scheduler 已消费），但 UI 表单只暴露 cron 表达式与固定间隔，
`CronTaskManagementView` 的表单类型甚至显式 `Exclude<ScheduleKind, 'at'>`。
本切片把 `at` 接进表单，让三类调度都能在 UI 创建/编辑：

- `ui/src/components/cron/CronTaskManagementView.tsx`：
  - 表单状态 `scheduleKind` 放开为完整 `ScheduleKind`，新增 `atValue`
    （datetime-local 字符串）；`openEdit` 按 `job.schedule.kind` 原样回填，
    `atMs` → 本地 `toDatetimeLocal` 格式。
  - 「运行方式」下拉新增「定时一次」；选中后出现 `datetime-local` 触发时间字段
    （`#cron-at`）。
  - 提交校验：空/非法时间 → `cron.errors.atTimeRequired`；不晚于当前时间 →
    `cron.errors.atTimeFuture`（UI 层强制未来时刻；后端正 `atMs` 语义不变）。
    提交载荷 `{ kind: 'at', atMs }`，走既有 `cron/create`|`cron/update`。
  - `formatSchedule` 对 `at` 任务显示「定时一次：<本地时间>」
    （`cron.scheduleFormat.onceAt`，替代原先无时间信息的 `once` 文案；`once`
    键随删，仅此一处引用）。
- `ui/src/i18n/en.ts`、`zh.ts`：`cron.atOption`/`atLabel`、
  `cron.errors.atTimeRequired`/`atTimeFuture`、`cron.scheduleFormat.onceAt`
  双语（en/zh）。
- `ui/e2e/cron-tasks.spec.ts`：新增离线规格——「定时一次」选项出现、选中后
  `datetime-local` 字段可见、过去时间提交被未来校验拦下（不发创建请求，
  不依赖真实供应商；既有主规格仍是 provider-gated）。

## What was explicitly not done（行内既定前置，不擅动）

- 错过调度的补跑（catch-up）——行文写明等 CH-0。
- `payload.deliver/channel/to` 外发通道——等 CH-0。
- cron 运行独立审批治理面——另行提案。

## Board

`docs/TODO.md` UI-CRON-P2 行内「at 表单入口」子项闭账；行整体保留 OPEN
（补跑/外发两个子项仍等 CH-0），行注更新。
