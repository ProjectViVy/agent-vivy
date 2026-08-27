# 验证记录 — 2026-08-28 provider-direct-write

范围：独立 worktree `feat/provider-direct-write`（根树正被 channels-ui 合并且
有未完成 merge，按 parallel-worktree-isolation 硬规则隔离开发）。

## 命令与结果

| 命令 | 结果 |
|---|---|
| `go build ./internal/app/settings/... ./internal/rpc/...` | ✅ |
| `go test -tags vivy_headless ./internal/app/settings/...` | ✅ 新增注册表 round-trip/校验/冲突/ActiveKey/Upsert 用例 |
| `go test ./internal/rpc/...` | ✅ 新增 settings/providers*、update 保注册表、写时 env 回调用例 |
| `go test -tags vivy_headless ./internal/config/...` | ✅ 用户主目录默认值（`t.Setenv(VIVY_USER_HOME, tmp)`） |
| `go test -tags vivy_headless ./internal/app/...` | ✅ （含 settings_overlay_test.go，gofmt 后） |
| `go test -tags vivy_headless ./...` | 除 `internal/eval`、`internal/studiocore`、`sdk/internal` 外全绿；这三者失败是**缺少 `ui/dist` 构建产物**（embed `all:dist`），非本次改动 |
| `cd ui; pnpm install --frozen-lockfile` | ✅ |
| `cd ui; pnpm typecheck` | ✅ 全绿 |
| `cd ui; pnpm test` | ✅ 122/122（custom-providers 重写 15、saved-models 18、store 4 等） |
| `cd ui; pnpm build` | ✅ 产出 `ui/dist`，之后 eval/studiocore/sdk 测试不再缺 embed |
| `just ci`（fmt-check + vet + test + headless-compile + ui-ci） | 见下 |

## just ci 最终结果

- 首跑被 `internal/app/settings_overlay_test.go` 未 gofmt 拦截（该文件为基线
  既有问题，`gofmt -w` 修复）。
- 修复后重跑：fmt-check ✅ · vet ✅ · `go test ./...`（无 tags）✅（eval/
  studiocore/sdk 因 `ui/dist` 现在已构建而不再失败）· headless-compile ✅ ·
  ui-ci（install/typecheck/test/build）✅。

## 已知偏差（如实记录）

- 浏览器冒烟（`http://127.0.0.1:3015`）：本 worktree 未起后端+前端进程对；
  且根树资源正被并行 lane 占用。按仓库惯例在 acceptance 中给出人工验收步骤，
  由用户在有空闲端口的会话里代验（与既往迭代一致）。