# Verification — UI-TRAJ / UI-TRAJECTORY-DEMO

日期：2026-09-02

## Kernel

| 命令 | 结果 |
| --- | --- |
| `gofmt -l ./internal/rpc ./internal/runtime` | 无输出（格式干净） |
| `go vet ./internal/rpc ./internal/runtime` | ok |
| `go test ./internal/rpc/ -run 'TestTrajectorySessionRoute' -count=1` | `ok agent-vivy/internal/rpc 0.572s` |
| `go test ./internal/runtime/ -run 'TestSessionTrajectory' -count=1` | `ok agent-vivy/internal/runtime 3.437s` |
| `just ci`（后台，tail-check） | `CI-EXIT:0`（见 /tmp/ci-traj.log） |

## UI

| 命令 | 结果 |
| --- | --- |
| `cd ui && pnpm typecheck` | 通过（tsc --noEmit 无输出） |
| `cd ui && pnpm test` | `24 files / 196 tests passed`（含 trajectory-utils 18 条） |

## Real-path smoke（真实控制面 + 浏览器）

- `cd ui && pnpm build` → 构建成功（7.29s）。
- 新增常驻 e2e `ui/e2e/trajectory-panel.spec.ts`（provider 无关，真实控制面）：
  新建会话 → 中控台 → 轨迹 tab → 断言 `data-trajectory-panel`、
  会话选择器 `data-trajectory-session-select`、无 run 会话的账本空态
  「暂无轨迹记录」。
- 单规格：`pnpm exec playwright test e2e/trajectory-panel.spec.ts` → 1 passed。
- 全套：`cd ui && pnpm e2e` → **18 passed / 1 skipped**（36.5s，含 runtime 真实
  对话回归与本规格），无既有规格回归。
