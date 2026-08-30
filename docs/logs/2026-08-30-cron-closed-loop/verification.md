# Verification

## Commands

From worktree `../agent-vivy-cron`（branch `feat/cron-closed-loop`，基于 main `ee2ba80`）：

```text
just ci          # fmt-check / vet / go test ./... / headless-compile / ui-ci
```

分层单测（开发期先行验证，均含在 `go test ./...` 内）：

```text
go test ./internal/runtime/  -run "TestCron|TestNextCronAfter|TestValidateCronSchedule" -count=1
go test ./internal/storage/... -count=1
go test ./internal/rpc/      -run "TestControlCron" -count=1
go test ./internal/config/   -run TestCronConfigDefaults -count=1
```

UI：

```text
cd ui; pnpm typecheck; pnpm test        # 176 vitest 全绿（含 api.test.ts 新 cron 断言、i18n zh/en 叶子奇偶）
pnpm exec playwright test cron-tasks    # 新增 e2e
pnpm exec playwright test               # 全量 e2e 回归
```

真路径冒烟（split pair；8787/3015 被根工作树 lane 占用，故用等价 split：专用后端 8791 + Vite 3016，`VIVY_BACKEND_ADDR=http://127.0.0.1:8791 pnpm exec vite --port 3016`）：

```text
# Playwright 真浏览器脚本：/cron-tasks 建「冒烟定时任务」→ 立即运行 → 等待"已完成" → 查看会话跳转 → 校验无 vivy.demo.* 键
node smoke-scratch.mjs   # 输出 SMOKE PASS；截图 1-cron-page/2-completed/3-session.png（见 acceptance.md）
```

## Results

- `just ci` 全绿：fmt-check / vet / `go test ./...`（含新增 cron 全部单测）/ headless-compile / ui-ci（typecheck + 176 vitest + `pnpm build`）。
- 新增 e2e `ui/e2e/cron-tasks.spec.ts` 通过（真实 `go run ./cmd/vivy` 后端 + mock 模型）：建任务（后端算出下次运行）→ 立即运行 → 终态回写「已完成」→ 专属会话按钮可用 → reload 持久化 → 删除；页面无 `vivy.demo.*` localStorage 键、无 DemoBanner。
- 全量 e2e 回归：6 通过、2 失败 —— `runtime.spec.ts`（断言聊天「画图」按钮）与 `welcome-wizard.spec.ts`（断言过期密钥文案）。**在 main 根工作树复跑同样失败**，为既有过期规格（§0.1 已有 UI-E2E-DRAW / UI-E2E-STALE 登记），非本迭代引入，本轮未修。
- 冒烟脚本输出：`demo banner count: 0 / created: ok / trigger -> completed: ok / session jump: ok / demo keys: [] / SMOKE PASS`。
- 冒烟后临时进程与 scratch 脚本均已清理；`ui/dist` 占位文件未入库（UI-CI-BOOTSTRAP 既有问题，`pnpm build` 后即为真实产物）。
