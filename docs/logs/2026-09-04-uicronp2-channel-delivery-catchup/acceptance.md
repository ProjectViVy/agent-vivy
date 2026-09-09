# UI-CRON-P2 Acceptance

Date: 2026-09-04

| Acceptance item | Evidence | Result |
|---|---|---|
| Channel outbound capability and split delivery (`ChannelHost.Deliver`) | `internal/channelhost/host.go` adds the public `Deliver` method; supports channel readiness checks and automatic RunesLimiter splitting; `internal/channelhost/deliver_test.go` all green | **PASS** |
| Kernel/channel isolation principle (D-007 quarantine wall) | `internal/runtime/service.go` defines the `ChannelDeliverer` abstraction; `runtime` has zero `channelhost` import statements; assembly injection occurs in `internal/app/app.go` | **PASS** |
| Async outbound delivery after scheduled-task completion | `internal/runtime/cron_scheduler.go` extracts the latest assistant summary at terminal settlement and delivers asynchronously with an independent 30s timeout; `cron_scheduler_test.go` `TestCronSettleDeliversOutboundWhenEnabled` passes | **PASS** |
| Recurring-task overdue catch-up characterization (Fire once on wake, skip storms) | `recoverCron` preserves overdue recurring tasks as due and runs them once after wake, then advances to the future period; `TestCronRecoveryPastDueRecurringJobFiresOnceOnWakeAndSkipsStorm` verifies no cumulative storm | **PASS** |
| Strict validation of control-plane RPC outbound parameters | `internal/rpc/control.go` rejects outbound requests without a specified channel or to; `internal/rpc/cron_rpc_test.go` `TestControlCronDeliverValidation` passes | **PASS** |
| Frontend dynamic channel list and outbound form | `ui/src/components/cron/CronTaskManagementView.tsx` dynamically loads compiled channels and provides a switch, channel dropdown, and target input, with edit repopulation and details display | **PASS** |
| Bilingual alignment and localization completeness | `ui/src/i18n/zh.ts` and `en.ts` add all outbound entries; `pnpm --prefix ui test` (deep i18n validation) passes | **PASS** |
| Interaction and offline E2E tests | `ui/e2e/cron-tasks.spec.ts` adds outbound-form toggling, linked-input, and form-submission validation; Playwright tests pass | **PASS** |
| Full gate validation (`just ci`) | `just ci` (code formatting, frontend type checking and build, vet, unit tests, packaging, plugin verification) all green | **PASS** |

Acceptance conclusion: all characterization and functionality for `UI-CRON-P2` scheduled-task outbound delivery and catch-up passed acceptance; the corresponding item in `docs/TODO.md` is closed.
