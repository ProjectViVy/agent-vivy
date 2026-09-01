# TFLAKE-CRON — cron delete-after-run 契约的确定性测试

## What changed

`TestCronAtJobDeletesAfterSuccessfulRun` 在 `just ci` 满载下偶发超时：该测试
端到端等待 异步 fire→run→watch→settle 管线在墙钟预算内完成删除，负载下
超预算即假失败（2026-08-31 已缓解：预算 5s→30s + 失败 dump settled 行）。

根治改为契约层：delete-after-run 的行为本体是 `settleCronRun` 的同步分支
（cron_scheduler.go AT + DeleteAfterRun + ok → DeleteCronJob），不再依赖
墙钟验证：

- 新增 `TestCronSettleDeletesSuccessfulAtJob` — 直接以构造的
  `cronActiveRun` 调 `settleCronRun(RunCompleted)`，同步断言行已删除且
  active 标记清理。零调度循环、零等待。
- 新增失败孪生 `TestCronSettleKeepsFailedAtJobDisabled` — failed 状态的
  delete_after_run 一次性任务**不得删除**（操作者需要行上的错误状态），
  断言保留 + 禁用 + `last_status=error`（代表性失败路径）。
- 既有端到端测试保留为接线金丝雀（含 30s 预算缓解不动）。

无生产代码改动；纯测试补强。

## Explicitly not done

- 未引入 fake clock（`CronSchedulerOptions.Now` 已存在供需要者使用；本行
  的 flake 源是管线墙钟预算而非定时精度，契约单测后金丝雀偶发超时不再
  掩盖真实回归）。
- 未改 `settleCronRun` 生产逻辑。
