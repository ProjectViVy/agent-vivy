# 验证记录 — 2026-08-27 「已选模型」快捷切换

## 自动化门禁

| 命令 | 结果 |
|---|---|
| `cd ui; pnpm exec vitest run src/components/settings/saved-models.test.ts`（新增用例先行） | ✅ 16 / 16 通过（含自定义事件与 storage 广播路径） |
| `cd ui; pnpm typecheck` | ✅ 无错误 |
| `cd ui; pnpm test`（vitest 全量） | ✅ 14 个文件 / 84 用例全过（含新增 `saved-models.test.ts` 16 用例；i18n zh/en 结构同步测试过，证明 zh/en 新增词条一一对应） |
| `just ci`（仓库根，= fmt-check + vet + go test + headless-compile + ui-ci[install/typecheck/test/build]） | ✅ 全绿，ui build `✓ built in 3.55s`（仅既有 chunk>500kB 警告，与本次改动无关） |

## 浏览器冒烟（split pair：`just run` :8787 + `cd ui; pnpm dev` :3015）

**跳过自测，由用户 Studio 调试代验。** 2026-08-27 执行本交付时
`127.0.0.1:8787`（`vivy-backend`）与 `127.0.0.1:3015`（本仓库 `pnpm dev`
Vite）均已被用户 Vivy Studio 调试会话占用（pid 22900 / 21516），按计划
不动用户会话、不自起 dev，浏览器路径的逐项验证交给用户 Studio 会话执行。

建议用户在 Studio 中按 `acceptance.md` 快速过一遍核心链路：
加入（设置页点模型 / 顶栏加书签）→ 顶栏出现 → 切换 → 移除 → 刷新后持久。

## 结论

`just ci` 全绿（含 i18n 结构同步测试），符合 `just-ci-is-the-gate`；
用户可见行为因端口被用户 Studio 会话占用而由用户代验，
`smoke-for-user-visible-change` 的验证记录即本文件此节 + `acceptance.md`。