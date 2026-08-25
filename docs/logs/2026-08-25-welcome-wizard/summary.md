# Welcome Wizard 移植总结

Date: 2026-08-25
Status: complete

## Outcome

把 Agent-Diva 的欢迎页面/首次配置向导移植为 Vivy 的首次使用引导
（`WelcomeWizard`）：用户第一次打开 Vivy 时自动弹出，引导设置默认模型，
完成后导航到聊天 / 模型设置 / 技能库。向导走真实 `settings/get`、
`settings/update` RPC，不收集任何 API 密钥（D-010，密钥只由运行环境注入）。

## Delivered

- `ui/src/hooks/use-welcome.ts` — 首访完成标记（localStorage
  `vivy.ui.welcome.completed`）+ 打开状态模块，复用 `use-theme.ts` 的
  module-state + `useSyncExternalStore` 模式；`use-welcome.test.ts` 5 个单测。
- `ui/src/components/layout/WelcomeWizard.tsx` — 3 步 React 向导
  （介绍 → 模型 → 完成），`_layout.tsx` 挂载，`fixed inset-0 z-[200]` 遮罩 +
  漂浮光斑；模型步骤通过 `saveSettings` 走真实 `settings/update`。
- `ui/src/routes/_layout.tsx` — 初始化完成后、无错误且未完成首访时自动
  `openWelcome()`；向导在应用根部渲染。
- `ui/src/routes/_layout.settings.tsx` — 校验 `?tab=` 参数为合法
  `SettingsTab`，支持向导 deep-link 到设置页模型分区。
- `ui/src/components/settings/SettingsView.tsx` — 通用分区新增「欢迎向导」
  重跑入口卡片；`initialTab` 支持路由指定初始分区。
- `ui/src/i18n/zh.ts` / `en.ts` — 新增 `welcome.*` 词条（结构一致，
  `i18n.test.ts` 强制校验）。
- `ui/src/styles.css` — `.welcome-float` 漂浮光斑动画，尊重
  `prefers-reduced-motion`。
- `ui/e2e/welcome-wizard.spec.ts` — 全流程 e2e：首访自动弹出 → 跳过 →
  完成标记 → 刷新抑制 → 设置页重跑 → 预填 → 保存 → 完成 → deep-link →
  再次刷新抑制。
- `ui/e2e/runtime.spec.ts` / `ui/playwright.config.ts` — 适配向导（跳过
  弹窗、localStorage 白名单），并把 e2e 浏览器上下文固定为 `zh-CN`
  （见 verification.md 的 e2e 修复说明）。

## 设计决策

- **provider 用运行时模型束名**：向导填的是 `openai / anthropic / mock`
  之一；DeepSeek 等 OpenAI 兼容服务通过 Base URL 网关接入，不引入
  自造 provider 名（后端校验只接受这三个 bundle 名）。预填链：
  `settings.provider || settings.config_provider || 建议值`。
- **不收集 secrets**：向导明确提示「API 密钥通过运行环境变量注入」，
  表单只有 provider / default_model / base_url。
- **不导入 `demo-api.ts`**：模型保存走真实 RPC。
- **完成即写标记**：`completeWelcome()` 写 localStorage 标记并关闭，
  跳过（skip）同样写标记；因此后续打开不再自动弹出，设置页「重新运行
  向导」可随时重看。
- **可复跑**：设置页通用分区提供重跑入口，且每次打开向导都从第 1 步
  重来、按当前真实设置重新预填。

## Explicitly not done

- 未在 UI 中收集/展示任何 provider 密钥（遵守产品约束，不引入
  `env_key` 以外的 secret 输入）。
- 未改动后端 RPC 契约；`settings/get` / `settings/update` 字段不变。
- 未实现多语言之外的向导文案差异（zh 为权威词典）。
