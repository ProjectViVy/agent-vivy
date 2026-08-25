# 2026-08-25 VIVY 界面 i18n 国际化支持

## 变更

为 Vivy 物种 UI 做完整的 i18n 支持，语言切换入口为「设置 → 语言」。全程零新增依赖，按 `hooks/use-theme.ts` 同构模式实现（模块级状态 + `useSyncExternalStore` + localStorage 持久化）。

### i18n 基础设施（`ui/src/i18n/`）

- `index.ts`：语言注册表（zh / en）、`t(key, params)` 点号路径查询 + `{{param}}` 插值、`useTranslation()` Hook（语言切换时订阅组件自动重渲染）、`setLocale()` / `getLocale()` / `dateTimeLocale()`、localStorage 持久化（键 `vivy.language`）、浏览器语言探测（localStorage 之后、zh→zh / en→en）、`document.documentElement.lang` 同步。
- `zh.ts`：权威词典（默认语言 zh，保证既有中文断言的测试不变）。`en.ts`：`Dictionary = typeof zh` 强类型对齐。
- `index.test.ts`：zh/en 叶子键一致性 + 数组长度对齐、缺项回退、key 兜底、插值、数组叶子点号索引、setLocale 持久化 + DOM lang、localeOptions 语言跟随。

### 文案迁移范围

- **通用/布局/导航/聊天组件**：侧边栏、顶栏、输入区、会话等全部经 `t()` 渲染。
- **视图**：approvals / audit / cron / lifecycle / masks / notebook / persona / planning / skills / 全部 demo 视图（DemoBanner、Memory、Dashboard、Mcp、TokenStats）与 ThemePicker / LanguagePicker。
- **lib 层**：`rpc.ts`、`runtime-config.ts`、`store.ts`、`run-subscription.ts`、`useSkills.ts` 的错误消息与运行态文案全部本地化；`demo-api.ts` 的演示数据种子改为首次写入时经 `t()` 生成。
- **演示数据策略**：已缓存的 `vivy.demo.*` 数据保持写入时语言（与用户数据一致），不随切换改写；调用时生成的新字符串（新会话默认名、错误、报告、回复）全部本地化。
- **关键修复**：`vitest.config.ts` 补 `@` 别名（此前 5 个 lib 测试文件因此无法解析 `@/i18n`）；`mask-catalog.ts` 的 capabilities 改用 `t()` key 兜底边界检测（数组不直接返回）。

### 明确未做（移交他人/后续）

- **SettingsView 接线**：`LanguagePicker.tsx` 已建好（点击即 `setLocale` → 全局切换 + 持久化）但**未挂载**进 `SettingsView.tsx` 语言页——该文件由并行工作流他人负责（`settingview 别人在改，你不用管`），本次不触碰。当前设置页「语言 预览」标签是对方预览实现，实测点击不改变全局文案。接线仅需在语言页挂载 `<LanguagePicker />`（3 行），已记入 `docs/TODO.md` §0.1 `UI-SET-I18N`。
- **对方文件**：`SettingsView.tsx`、`DivaSettingsPreview.tsx`、`diva-preview-data.ts` 及其测试均未改动。
- **`reveal-engine.ts`**：模板文件（标注「业务代码请勿修改」），仅内部 `console.warn`，未动。
- 其余 CJK 仅存在于代码注释与 demo 关键词匹配（`sendMessage` 的中文关键词，属数据匹配而非文案）。
