# 验证记录 — 2026-08-27 自定义供应商注册表

## 自动化门禁

| 命令 | 结果 |
|---|---|
| `cd ui; pnpm exec vitest run src/components/settings/custom-providers.test.ts`（新增用例先行） | ✅ 16 / 16 通过 |
| `cd ui; pnpm exec vitest run src/components/settings/saved-models.test.ts src/components/settings/custom-providers.test.ts` | ✅ 34 / 34 通过（含更新后的标签回退断言） |
| `cd ui; pnpm typecheck` | ✅ 无错误 |
| `cd ui; pnpm test`（vitest 全量） | ✅ 15 个文件 / 102 用例全过（含新 custom-providers.test.ts 16 用例、更新后的 saved-models.test.ts；i18n zh/en 结构同步测试过） |
| `just ci`（仓库根，= fmt-check + vet + go test + headless-compile + ui-ci[install/typecheck/test/build]） | ✅ 全绿，ui build `✓ built in 4.06s`（仅既有 chunk>500kB 警告，与本次改动无关） |

## 浏览器冒烟（split pair：`just run` :8787 + `cd ui; pnpm dev` :3015）

**跳过自测，由用户 Studio 调试代验。** 2026-08-27 执行本交付时
`127.0.0.1:8787`（`vivy-backend`）与 `127.0.0.1:3015`（本仓库 `pnpm dev`
Vite）仍被用户 Vivy Studio 调试会话占用（pid 22900 / 21516，与上一交付相同），
按计划不动用户会话、不自起 dev，浏览器路径的逐项验证交给用户 Studio 会话执行。

建议用户在 Studio 中按 `acceptance.md` 快速过一遍核心链路：

1. 新增自定义供应商（显示名 + Base URL + 模型列表）→ 左栏出现带「自定义」
   标记的行；
2. 点该行 → 右栏显示其模型列表；点某模型 → 快捷列表出现 chip、顶栏立即切换；
3. 重命名显示名 → 所有已保存书签与顶栏标签同步变；
4. 删除供应商 → 书签仍在（标签回退 Base URL 主机名）、运行配置不变；
5. 重复 Base URL 录入被拦（显示名内校验报错）；
6. 刷新后注册表与快捷列表均持久。

## 结论

`just ci` 全绿（含 i18n 结构同步测试与 ui build），符合 `just-ci-is-the-gate`；
用户可见行为因端口被用户 Studio 会话占用而由用户代验，
`smoke-for-user-visible-change` 的验证记录即本文件此节 + `acceptance.md`。
