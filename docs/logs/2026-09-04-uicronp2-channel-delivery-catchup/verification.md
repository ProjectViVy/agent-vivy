# UI-CRON-P2 Verification

Date: 2026-09-04

## Verification Commands & Status

| Command | Target / Scope | Result |
|---|---|---|
| `just ci` | 全量 CI 门禁（fmt-check, ui-ci, vet, test, headless-compile, plugin-ci） | **PASS** |
| `pnpm --prefix ui e2e ui/e2e/cron-tasks.spec.ts` | Playwright E2E（离线 Mock 模式校验外发开关、联动输入及必填校验） | **PASS** (2 passed, 1 skipped) |
| `go test -v ./internal/channelhost -run TestDeliver` | 通道外发多片投递、未启动拦截、未注册通道单测 | **PASS** |
| `go test -v ./internal/runtime -run 'TestCronSettleDeliversOutboundWhenEnabled\|TestCronRecoveryPastDueRecurringJobFiresOnceOnWakeAndSkipsStorm'` | 任务完成异步投递总结、逾期周期任务唤醒仅单次执行并跳过风暴 | **PASS** |
| `go test -v ./internal/rpc -run TestControlCronDeliverValidation` | RPC 参数层对 `deliver: true` 时 channel/to 必填性校验 | **PASS** |
| `pnpm --prefix ui test` | UI i18n 双语字典键对齐与前端单测 | **PASS** |

## Test Evidence & Details

### 1. ChannelHost.Deliver 单测
- `internal/channelhost/deliver_test.go`:
  - `TestDeliverSplitsRunesWhenLimiterConfigured`: 模拟 `plugin.RunesLimiter`，验证超长文本安全切片并调用适配器 `Send`。
  - `TestDeliverFailsWhenChannelNotRunning`: 通道未启动（未配或关闭）时，返回 `channel %s is not running`。
  - `TestDeliverFailsWhenChannelNotRegistered`: 未注册通道安全报错。

### 2. Runtime Cron 调度与外发单测
- `internal/runtime/cron_scheduler_test.go`:
  - `TestCronSettleDeliversOutboundWhenEnabled`:
    - 配置 Mock `ChannelDeliverer`；
    - 执行一个带 `deliver: true, channel: "telegram", to: "12345"` 的 Cron 任务；
    - 验证结算后调用 `ChannelDeliverer.Deliver(ctx, "telegram", "12345", content)` 并包含会话总结；
    - 验证外发在独立 context 中异步执行，不阻断调度器主循环。
  - `TestCronRecoveryPastDueRecurringJobFiresOnceOnWakeAndSkipsStorm`:
    - 任务配置为每 10 秒执行一次周期任务；
    - 关机时间长达 60 秒（中间错失 6 个周期）；
    - 启动唤醒时，调度器恰好执行一次（`wakeRuns == 1`），下一次调度时间正确步进至未来周期（`nextRun > nowMs`），成功避免补跑雪崩（no cumulative storm）。

### 3. RPC 参数校验单测
- `internal/rpc/cron_rpc_test.go`:
  - `TestControlCronDeliverValidation`:
    - 传入 `payload.deliver = true` 但缺失 `channel` 或 `to` 时，断言返回 `-32602 (InvalidParams)`。

### 4. E2E 界面交互
- `ui/e2e/cron-tasks.spec.ts`:
  - 验证任务创建弹窗中的「结果外发至通道」开关切换；
  - 验证展开后的通道选择框（下拉选项来自已编译通道）和目标地址输入框；
  - 验证开启后未填写的客户端阻止提交提示。
