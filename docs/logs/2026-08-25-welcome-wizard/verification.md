# Verification

Date: 2026-08-25

## Gate: `just ci`（仓库根目录）

```
just ci
```

结果 **EXIT=0**，全部通过：

- Go：fmt-check / vet / test / headless-compile 干净
- UI typecheck：`tsc --noEmit` 干净（`src/**`）
- UI 单测：**51 passed（11 个测试文件）**，含新增
  `src/hooks/use-welcome.test.ts`（5 个用例）
- `vite build` 成功（仅有既存的 chunk-size 提示，非本次引入）

## E2E：`just ui-e2e`

```
just ui-e2e   # pnpm build + playwright test（自带后端 127.0.0.1:8799，mock runtime）
```

结果 **2 passed (15.7s)**：

- `welcome-wizard.spec.ts` — 首访自动弹出 → 跳过 → 完成标记 `'1'` →
  刷新抑制 → 设置页「重新运行向导」→ 模型步骤预填 `mock / mock:hitl` →
  保存 `mock / mock` → 完成步骤 → 「模型设置」deep-link 到
  `/settings?tab=model`（tab 选中、Provider 值 `mock`）→ 再次刷新抑制
- `runtime.spec.ts` — 既有控制面全流程回归通过（含新增的向导跳过）

### e2e 修复记录（本迭代内发现并修复，非遗留）

1. **浏览器语言上下文失配**：Playwright 默认上下文为 `en-US`，而应用
   `detectInitialLocale()` 会据此渲染英文界面；既有 spec 的中文选择器
   （`设置`、`输入消息...` 等）全部失效。修复：`playwright.config.ts`
   的 `use` 增加 `locale: 'zh-CN'`，e2e 稳定模拟中文用户。
2. **向导预填断言**：e2e 后端 `runtime.mock: true` 使生效 provider 为
   `mock`、默认模型 `mock:hitl`（`defaultModelFor` 按 mock scenario 派生），
   而不是配置里的 `active: openai`。修正断言为 `mock / mock:hitl`。
3. **过时断言「返回列表」**：`MasterDetail` 重构后返回按钮文案为
   `t('common.back')`（「返回」），`runtime.spec.ts` 三处 `返回列表`
   已过时。修正为「返回」。（独立于本功能的历史断链，本次一并修复，
   否则整套 e2e 无法转绿。）

## 真实路径冒烟：http://127.0.0.1:3015（split Vite）

在开发者浏览器（IAB）逐项人工验证，全部符合预期：

1. 设置页 → 通用分区：显示「欢迎向导」重跑入口卡片与「重新运行向导」按钮。
2. 点击「重新运行向导」：向导以中文弹出，介绍步骤渲染品牌
   （Vivy / Project ViVY）、副标题、三步指示器（开始 / 模型 / 完成）、
   三项特性（智能对话 / 技能扩展 / 生命周期与进化）、「跳过向导」。
3. 点击「下一步」：模型步骤渲染，Provider / 默认模型从真实后端设置预填
   （`mock / mock`），Base URL 空、显示占位提示；「API 密钥通过运行环境
   变量注入，不在界面中填写或保存」提示与「在浏览器中打开」按钮正常。
4. 点击「下一步」：完成步骤渲染「准备就绪！」与三张导航卡片
   （开始聊天 / 模型设置 / 技能库）。
5. 点击「模型设置」：向导关闭（写完成标记），URL 变为
   `http://127.0.0.1:3015/settings?tab=model`，设置页「模型」tab 选中，
   Provider 输入框值为保存的 `mock`。

说明：开发期后端（`just dev` 对）无 `OPENAI_API_KEY`，冒烟停留在
mock provider；向导在真实 provider 下的保存路径与错误内联提示
（`settings: provider ... unsupported; want openai, anthropic or mock`）
已在开发冒烟中验证过错误分支展示。

## 跳过说明

- 未做 embedded-UI（:8787）冒烟：开发/验证路径按 AGENTS.md 使用 split
  Vite（:3015）；embedded UI 由 `just ci` 的 `vite build` 覆盖编译，
  e2e 的 `go run ./cmd/vivy`（:8799，embedded UI）已完整回归向导流程。
