# Verification — UI-TRAJ / UI-TRAJECTORY-DEMO

Date: 2026-09-02

## Kernel

| Command | Result |
| --- | --- |
| `gofmt -l ./internal/rpc ./internal/runtime` | no output (clean formatting) |
| `go vet ./internal/rpc ./internal/runtime` | ok |
| `go test ./internal/rpc/ -run 'TestTrajectorySessionRoute' -count=1` | `ok agent-vivy/internal/rpc 0.572s` |
| `go test ./internal/runtime/ -run 'TestSessionTrajectory' -count=1` | `ok agent-vivy/internal/runtime 3.437s` |
| `just ci` (background, tail-check) | `CI-EXIT:0` (see `/tmp/ci-traj.log`) |

## UI

| Command | Result |
| --- | --- |
| `cd ui && pnpm typecheck` | passed (`tsc --noEmit` produced no output) |
| `cd ui && pnpm test` | `24 files / 196 tests passed` (including 18 trajectory-utils tests) |

## Real-path smoke (real control plane + browser)

- `cd ui && pnpm build` → build succeeded (7.29s).
- Added permanent e2e `ui/e2e/trajectory-panel.spec.ts` (provider-independent, real control
  plane): create session → Console → Trajectory tab → assert `data-trajectory-panel`, the
  session selector `data-trajectory-session-select`, and the ledger empty state for a
  session with no run, "No trajectory records".
- Single spec: `pnpm exec playwright test e2e/trajectory-panel.spec.ts` → 1 passed.
- Full suite: `cd ui && pnpm e2e` → **18 passed / 1 skipped** (36.5s, including the runtime
  real-conversation regression and this spec), with no regression in existing specs.
