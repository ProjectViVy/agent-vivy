# UI-CRON-P2 Acceptance

Date: 2026-09-04

| Acceptance item | Evidence | Result |
|---|---|---|
| 通道外发能力与分片投递 (`ChannelHost.Deliver`) | `internal/channelhost/host.go` 新增公开 `Deliver` 方法；支持通道就绪状态检查与 RunesLimiter 自动切片；`internal/channelhost/deliver_test.go` 全绿 | **PASS** |
| 内核与通道隔离原则 (D-007 检疫墙) | `internal/runtime/service.go` 定义抽象接口 `ChannelDeliverer`，`runtime` 零 import `channelhost`；在 `internal/app/app.go` 装配注入 | **PASS** |
| 定时任务执行完异步外发 | `internal/runtime/cron_scheduler.go` 在结算终态提取最新助手总结，以 30s 独立超时异步投递；`cron_scheduler_test.go` `TestCronSettleDeliversOutboundWhenEnabled` 验证通过 | **PASS** |
| 周期任务到期补跑定性 (Fire once on wake, skip storms) | `recoverCron` 对逾期周期任务保持到期态并在唤醒后执行一次，随即步进至未来周期；`TestCronRecoveryPastDueRecurringJobFiresOnceOnWakeAndSkipsStorm` 验证无累积雪崩 | **PASS** |
| 控制面 RPC 外发参数强校验 | `internal/rpc/control.go` 拦截未指定 channel 或 to 的外发请求；`internal/rpc/cron_rpc_test.go` `TestControlCronDeliverValidation` 验证通过 | **PASS** |
| 前端动态通道列表与外发表单 | `ui/src/components/cron/CronTaskManagementView.tsx` 动态加载已编译通道列表，提供开关、通道下拉框及目标输入框，支持编辑回填与详情展示 | **PASS** |
| 双语对齐与国际化完整性 | `ui/src/i18n/zh.ts` 与 `en.ts` 补充所有外发词条，`pnpm --prefix ui test` (i18n 深度校验) 通过 | **PASS** |
| 交互与离线 E2E 测试 | `ui/e2e/cron-tasks.spec.ts` 新增外发表单切换、联动输入与表单提交验证，Playwright 测试通过 | **PASS** |
| 全量门禁校验 (`just ci`) | `just ci`（代码格式、前端类型检查与构建、vet、单测、打包、插件验证）全绿 | **PASS** |

验收结论：`UI-CRON-P2` 定时任务外发与补跑全部定性与功能均已通过验收，`docs/TODO.md` 对应项已关闭。
