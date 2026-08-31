# Verification — VC-1f

Worktree `agent-vivy-vc0`（分支 `feat/vc1a-bash-tool`），2026-08-31。

| # | 命令 | 结果 |
|---|------|------|
| 1 | `go build ./...` | exit 0（服务端 go-udiff 接入后） |
| 2 | `go test ./internal/runtime -run 'TestBackendPatch\|TestBackendMultiPatch\|TestFilesystemBackend\|TestPatch'` | exit 0（2 处 diff 断言更新后） |
| 3 | `cd ui; pnpm typecheck` | exit 0 |
| 4 | `cd ui; pnpm test` | 24 files / 190 tests 全绿（含新增 diff.test.ts 12 例 + DiffView.test.tsx SSR 2 例；期间修正 1 处测试自身预期错误：1 del + 2 adds 按位置配对为 2 行而非 3 行） |
| 5 | `just ci` | 第 1 次运行在 `go test ./...` 阶段失败（internal/runtime，90s，未捕获具体用例名）；单独重跑 `go test ./internal/runtime` 与全量 `go test ./...` 均全绿（26 包，0 FAIL），判定为并行满载下的计时型 flake，非本次改动引入 |
| 6 | `just ci`（重跑，真实退出码直采） | **exit 0**（fmt-check / vet / go test / headless-compile / ui-ci 全过；后台任务 b5s2zi6fc） |
| 7 | `cd ui; pnpm e2e` | 5 passed / 1 skipped / **3 failed**。3 个失败（language-setting、model-refresh、welcome-wizard）均为 strict-mode 定位符过期，与本交付无关：失败元素（`切换模型` 按钮、`gpt-4o-mini` 双处文本、`配置模型` 标题）位于本交付未触碰的 Settings/向导组件，且 `docs/TODO.md` §0.1 `E2E-STALE` 已跟踪同一组失败（2026-08-30 发现、2026-08-31 remove-runtime-mock 迭代复证「干净 HEAD 上同样失败」）。关键回归面 `runtime.spec.ts`（真实浏览器 + 真实后端聊天页）通过 |

## Smoke policy note（3015 浏览器冒烟）

Diff 渲染的完整浏览器冒烟依赖一次真实文件变更 run：产生该 run 需要已配置的
供应商密钥（写审批/工具结果都由模型驱动）。本环境无密钥，与 VC-1e 的冒烟
政策说明一致，处理方式：

- 组件渲染路径用 vitest SSR（`renderToStaticMarkup`）直接渲染真实 DiffView
  组件（含 i18n 与统计/切换按钮），断言 hunk 头、`+TWO`、`新增 1 行，删除 1 行`
  aria 标签与统一/分栏按钮文案 —— 覆盖组件真实 render，而非仅解析器。
- Playwright e2e 的 runtime.spec 在真实浏览器 + 真实后端下跑通聊天页回归，
  确认 MessageBubble 改动未破坏既有交互（复制/重新生成/悬停操作栏等）。
- 有密钥环境的人工验收路径见 acceptance.md。
