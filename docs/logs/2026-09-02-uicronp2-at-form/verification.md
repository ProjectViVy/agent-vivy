# Verification — UI-CRON-P2（at 表单）

Commands run from the repository root (`agent-vivy/`):

1. `cd ui && pnpm typecheck` — clean（`Exclude<ScheduleKind, 'at'>` 放开为
   `ScheduleKind` 后无类型回归）。
2. `just ci` — 后台整跑，tail 检查日志 `CI-EXIT:0`（含 ui-ci typecheck/test/build、
   全部 Go 包测试；本切片未动 Go 代码，Go 侧为回归面）。
3. `just ui-e2e` — 后台整跑，tail 检查 `E2E-EXIT:0`；确认
   `cron-tasks.spec.ts` 两条规格（既有 provider-gated 主规格 + 新离线 at 表单
   规格）通过。

（结果在提交前回填核对：CI-EXIT:0 / E2E-EXIT:0 实测见 git 提交信息对应的
verification 追加。）
