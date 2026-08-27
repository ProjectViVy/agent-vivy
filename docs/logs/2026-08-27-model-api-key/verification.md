# 验证记录 — 2026-08-27 模型密钥端到端支持

## 自动化门禁

| 命令 | 结果 |
|---|---|
| `go test -count=1 ./internal/app/... ./internal/rpc/...` | ✅ 全过（settings round-trip+校验、RPC api_key 写入/清除/不泄露、app overlay 注入/空值/缺档三个新用例） |
| `cd ui; pnpm typecheck` | ✅ 无错误 |
| `cd ui; pnpm test`（vitest 全量） | ✅ 15 个文件 / 105 用例全过（含新增 `apiKey`/`customApiKeyFor` 3 用例、旧条目缺省补空串用例；i18n zh/en 结构同步测试过） |
| `just ci`（仓库根，= fmt-check + vet + go test + headless-compile + ui-ci[install/typecheck/test/build]） | 首次因 `settings_overlay_test.go` 未 gofmt 被 fmt-check 拦下；`gofmt -w` 后 ✅ 全绿——ui build `✓ built in 3.53s`，105 用例全过（仅既有 chunk>500kB 警告） |

## 浏览器冒烟（split pair：`just run` :8787 + `cd ui; pnpm dev` :3015）

**跳过自测，由用户 Studio 调试代验。** 2026-08-27 执行本交付时
`127.0.0.1:8787`（`vivy-backend`）与 `127.0.0.1:3015`（本仓库 `pnpm dev`
Vite）仍被用户 Vivy Studio 调试会话占用（pid 22900 / 21516），按计划不动用户
会话、不自起 dev，浏览器路径的逐项验证交给用户 Studio 会话执行。

建议用户在 Studio 中按 `acceptance.md` 快速过一遍核心链路：

1. 新增自定义供应商时填写 API Key；
2. 点击其模型 → `data/agent-home/settings.yaml` 落盘含 `api_key`；
3. 顶栏/chip 切换带密钥；切换回目录模型后密钥清除（回落 env）；
4. 刷新页面后「已配置 API Key」提示仍在；`settings/get` 只回 `api_key_set`。

## 结论

`just ci` 全绿（fmt-check 拦截 → gofmt 修复后通过），符合 `just-ci-is-the-gate`；
用户可见行为因端口被用户 Studio 会话占用而由用户代验，
`smoke-for-user-visible-change` 的验证记录即本文件此节 + `acceptance.md`。
