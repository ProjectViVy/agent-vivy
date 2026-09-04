# UI-CRON-P2 — 定时任务外发通道与补跑闭环

## What changed

在 2026-09-02 完成 `at`（定时一次）表单入口后，本切片彻底完成 `UI-CRON-P2` 剩余的「外发通道接线」与「补跑（Catch-up）定性闭环」，使得定时任务体系全面闭环：

### 1. 通道宿主与内核隔离外发 (`ChannelHost.Deliver` + `runtime.ChannelDeliverer`)
- **`internal/channelhost/host.go`**：
  - 新增公开方法 `Deliver(ctx context.Context, channelName, chatID, content string) error`。
  - 严格校验通道是否已编译且当前处于已启动状态（`started`），未启动返回安全且携带说明的错误。
  - 通过 `plugin.RunesLimiter` 接口感知适配器平台字符限制，利用 `splitRunes` 智能分片推送。
  - 单测 `internal/channelhost/deliver_test.go` 覆盖正常多片投递与未启动/未注册等异常拦截。
- **`internal/runtime/service.go`**：
  - 定义解耦接口 `ChannelDeliverer`，`internal/runtime` 零 import `internal/channelhost`，完全遵循 D-007 隔离原则。
  - `ServiceDeps` 增加可选 `Channels ChannelDeliverer`。
- **`internal/app/app.go`**：
  - 装配层在实例化 `runtime.NewService` 时将 `channelHost` 注入 `ServiceDeps.Channels`。
- **`internal/runtime/cron_scheduler.go`**：
  - `settleCronRun` 终态结算：当任务 `payload.deliver === true` 时，若执行成功提取专属会话中该 run 的最新一条助手消息作为总结；若失败则提取终端错误。
  - 在独立 goroutine 中通过带有 `30s` 强超时的 context 异步调用 `s.deps.Channels.Deliver`，绝不阻塞调度结算与时钟轮。
  - `cron_scheduler_test.go` 新增 `TestCronSettleDeliversOutboundWhenEnabled` 确定性断言。

### 2. 补跑（Catch-up）定性闭环 (Fire once on wake, skip storms)
- **`internal/runtime/cron_scheduler.go`**：
  - 启动恢复 `recoverCron`：
    - 单次调度（`at`）：过去时间已过直接标记为已停用（`Enabled = false, NextRunAtMs = 0`），保持既有安全语义；
    - 周期调度（`every` / `cron`）：若 `0 < NextRunAtMs <= nowMs`（停机期间已到期），维持其到期状态，不提前跳跃到未来。调度器主循环 `fireDueCronJobs` 首次唤醒时触发**恰好一次**执行，随后 `settleCronRun` 自动步进至未来的 `NextCronAfter(schedule, nowMs)`，跳过停机期间的所有中间错失周期。
  - `cron_scheduler_test.go` 新增 `TestCronRecoveryPastDueRecurringJobFiresOnceOnWakeAndSkipsStorm` 确定性单测。

### 3. 控制面参数校验 (`internal/rpc/control.go`)
- `buildCronJob`：当 `payload.Deliver === true` 时，强制校验 `payload.Channel` 与 `payload.To` 非空，否则返回 `-32602 (InvalidParams)`。
- `cron_rpc_test.go` 新增 `TestControlCronDeliverValidation`。

### 4. 前端表单与通道下拉框 (`ui/`)
- **`ui/src/components/cron/CronTaskManagementView.tsx`**：
  - 启动时自动通过 `inspectChannels()` 动态获取已编译通道列表。
  - 新建/编辑弹窗中增加「结果外发至通道」开关，开启后展开通道下拉框（`Select`，展示已编译平台名称）与接收目标输入框（`Input`）。
  - 编辑已有任务时完整回填 `deliver`、`channel`、`to` 状态。
  - 详情面板增加「结果外发」信息展示（如 `Telegram · 目标：12345` 或 `未启用`）。
  - 客户端校验：开启外发时必须选择通道并输入接收目标。
- **`ui/src/i18n/zh.ts` & `en.ts`**：
  - 补充中英双语词条，严格保持字典键对齐（通过 `i18n.test.ts` 校验）。
- **`ui/e2e/cron-tasks.spec.ts`**：
  - 新增离线 Playwright E2E 规格断言外发开关交互、输入框可见性与必填校验。

---

## What was explicitly not done

- 未引入繁琐的分布式队列与积压补跑风暴机制（按架构定性明确采用 `fire once on wake, skip storms`）。
- 未破坏通道检疫墙（`internal/runtime` 保持零 channelhost/plugin/eino 依赖）。
