# 验证记录 — 生成参数演示收进「设置 → 通用 → 高级特性」

工作分支：`feat/settings-genparams-provider`（worktree 开发，根树保持另一条
lane 的未提交改动不动）。改动经过一次方向推翻（provider 面板版 → 通用高级特性
按模型版），交付前 `git reset --soft` 重组为最终单一提交。

## 命令与结果

| 步骤 | 命令 | 结果 |
| --- | --- | --- |
| 全量门禁（先 `pnpm build` 产出真实 `ui/dist` 满足 go:embed） | `just ci` | ✅ 通过（fmt-check / vet / go test / headless-compile / typecheck / vitest / vite build） |
| 新增单测（演示生成参数按模型独立） | `pnpm test`（ci 内 `demo-api.test.ts` 11 tests） | ✅ 通过 |
| 浏览器冒烟（内嵌 UI 路径） | `pnpm exec playwright test e2e/genparams-advanced.spec.ts` | ✅ 1 passed |

## 浏览器冒烟（Playwright，真实浏览器走通）

新增 `ui/e2e/genparams-advanced.spec.ts`（随交付提交，作回归规格），对
`http://127.0.0.1:8799`（e2e 自建后端 + 内嵌本次构建的 `ui/dist`）实走：

1. `/settings` 通用 Tab：出现「高级特性」卡；全页无独立卡片标题级「生成参数」
   heading。
2. 预置两个已选模型（gpt-4o-mini / gpt-4o）后，下拉默认选中 gpt-4o-mini：
   温度 0.7、最大 Tokens 4096、提示「正在编辑 gpt-4o-mini 的生成参数。」。
3. 保存 8192 →「已保存到本地」；`vivy.demo.gen-params` 键
   `openai/https://api.openai.com/v1/gpt-4o-mini` 写入。
4. 下拉切到 gpt-4o → 载入默认 4096，gpt-4o-mini 键不受影响；保存 1024 后两键
   并存互不覆盖。
5. 刷新 → 默认仍选中 gpt-4o-mini，读取已保存的 8192；`vivy.demo.gen-params`
   持久化。

## 说明

- 根树 :3015/:8787 属于另一条并行 lane（压缩分区并入通用），本分支占用
  `ui/e2e` 自建 8799 端口冒烟，未干扰根树。
- `just ci` 在「无真实 `ui/dist`」的新 worktree 首跑会失败
  （`go:embed all:dist` / embed 测试 503），先 `pnpm build` 后全绿——环境
  缺口，非代码问题。
- 既有问题不属本次改动（照录共用看板）：`ui/e2e/runtime.spec.ts:84` 与
  `welcome-wizard.spec.ts:33` 断言过期文案，见 `docs/TODO.md` §0.1
  `UI-E2E-STALE`。