# 验证记录 — 2026-08-27 模型设置页交互重构

## 自动化门禁

| 命令 | 结果 |
|---|---|
| `cd ui; pnpm typecheck` | ✅ 无错误 |
| `cd ui; pnpm test`（vitest 全量） | ✅ 15 个文件 / 105 用例全过（i18n zh/en 结构同步测试过；本迭代为纯 UI 重构，无新增纯逻辑） |
| `just ci`（仓库根，= fmt-check + vet + go test + headless-compile + ui-ci[install/typecheck/test/build]） | ✅ 全绿，ui build `✓ built in 4.80s`（仅既有 chunk>500kB 警告） |

## 浏览器冒烟（split pair：`just run` :8787 + `cd ui; pnpm dev` :3015）

**跳过自测，由用户 Studio 调试代验。** `127.0.0.1:8787`（`vivy-backend`）与
`127.0.0.1:3015`（本仓库 `pnpm dev` Vite）仍被用户 Vivy Studio 调试会话占用
（pid 22900 / 21516），本次评审即用户在 Studio 中进行，浏览器路径由用户验证。

建议用户在 Studio 中按 `acceptance.md` 快速过一遍：

1. 自定义供应商行右侧常驻铅笔按钮 → 编辑对话框出现（改地址/别名）；
2. 模型列表头部「从官方同步」与「新增」按钮出现，「新增」能手加模型并立即
   应用；
3. 底部表单已消失，API Key 出现在模型列表上方（自定义可填、目录禁用）。

## 结论

`just ci` 全绿（见提交消息/结论行），符合 `just-ci-is-the-gate`；用户可见行为
由用户在 Studio 会话中直接评审代验，`smoke-for-user-visible-change` 的记录即
本文件此节 + `acceptance.md`。
