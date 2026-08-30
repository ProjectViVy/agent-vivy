# Cron 闭环（UI CRON 面板接真实调度后端）

日期：2026-08-30 · 分支：`feat/cron-closed-loop`（worktree `../agent-vivy-cron`）· 参考：diva 实现（`ui/agent-diva-source/agent-diva-core/src/cron/`）

## 交付了什么

`/cron-tasks` 面板从 localStorage 演示页（`vivy.demo.cron-jobs` + DemoBanner）升级为真实闭环：
UI 建/改/删/启停任务 → Go 后端 armed-timer 调度器到点自动触发 → 任务的 message 在其专属会话里驱动一次 agent run → run 终态回写 `lastRun/lastStatus/lastError/nextRun` → 面板 5s 轻轮询实时展示，并可一键跳转到任务会话（运行中走 `background/attach` 实时看，历史走 `selectSession`）。

调度语义对齐 diva：

- 三种 schedule：`at`（一次性时间戳）/ `every`（间隔）/ `cron`（5 段表达式 + tz；6 段仅接受秒位 0，等价 diva 的 `0 <5段>` 归一化）。
- armed-timer 睡到最近触发点（30s 兜底重算）；**错过不补跑**；重启时重算 next，过期 `at` 任务自动停用。
- 同一任务同时只允许一个运行（重复触发/二次调度跳过，RPC 层映射 -32009 Conflict）。
- `at` 任务跑完自动停用；`deleteAfterRun=true` 且成功则删除任务行。
- 手动触发（`cron/trigger`）不限 enabled；调度 fire 仅限 enabled。
- 终态回写：watcher 轮询 run 行（bus 终态发布只关订阅通道不投递事件，故不订阅 bus），`run.completed→ok`、`run.failed→error+journal 失败文案`、`run.cancelled→error`。
- cron 表达式为纯 Go 自实现解析器（`internal/runtime/cronexpr.go`；沙箱不出网、robfig 不在模块缓存）；`import _ "time/tzdata"` 内嵌 IANA 库保证 Windows 可用。

## 组件清单

- **domain**：`internal/domain/cron.go`（CronJob/Schedule/Payload/State；JSON tag 服务于双后端共用的 schedule_json/payload_json 列）。
- **storage**：契约 `CronStore` 挂入 `Engine`；SQLite `migration016`（cron_jobs 表 + enabled/next_run 索引）+ `sqlite/crons.go`；Postgres bootstrap 升 `schemaV15`（版本常量 15）+ `postgres/crons.go` 对等实现。
- **runtime**：`cronexpr.go` 解析/Next/校验；`cron_scheduler.go` 调度器（`StartCronScheduler/StopCronScheduler/KickCronScheduler` + `CronRunner{TriggerCron,StopCron,ActiveCronRun}`），`ServiceDeps.Crons` 注入，单 organism（租约）内安全。
- **rpc**：`cron/list|create|update|delete|trigger|stop` 六方法 + capabilities `cron.*`；wire DTO 逐字段对齐 UI `CronJobDto`（diva 混用大小写约定），新增 Vivy 扩展字段 `sessionId`；错误映射 404/-32009/-32602。
- **app**：`runtime.cron.enabled`（默认 true）开关；`App.Run` 启动调度器，关停序列在 `StopInteractionSweeper` 后、`CancelAll` 前排空（watcher 有界 3s，超时容忍）。
- **ui**：`api.ts` 新增 6 个方法与 Cron 类型（类型迁出 types.ts demo 区）；`CronTaskManagementView` 换真实 RPC、开关改整对象更新、去演示 payload kind 选择、5s 轮询、查看会话/查看运行按钮、最近错误展示；路由去 DemoBanner；i18n zh/en 同步改写并删除 `demo.cron.*`；`demo-api.ts` 的 cron 模拟块整体移除。
- **tests**：解析器表测（含时区/不可达表达式）、调度器行为测（触发回写/at 停用/删除/恢复/冲突/取消）、SQLite CRUD、RPC wire+校验+404/409、config 默认值、`ui/e2e/cron-tasks.spec.ts`（真实后端全流程）。

## 明确不做（本轮范围外）

- `deliver/channel/to` 的真实外发语义（仅存储；Vivy 无外发通道）。
- 错过调度的补跑（catch-up）；跨进程/分布式调度（单 organism 租约已保证单实例）。
- `at` 类型在表单中的入口（后端/API 支持，UI 表单暂只暴露 cron/every）。
- cron 运行的独立审批策略（沿用会话默认 smart 预设；HITL 待办照旧）。

以上留待 `docs/TODO.md` §0.1 `UI-CRON-P2`。
