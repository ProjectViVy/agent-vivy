# 2026-08-27 · Agent-Diva 供应商目录与折叠逻辑移植（前端）

## 变更内容

把 Agent-Diva 的模型供应商目录及其"一批不常用供应商默认折叠"的交互逻辑
移植到 Vivy 前端，落地在设置页「模型」Tab 与顶栏模型切换器。

### 新增

- `ui/scripts/gen-provider-catalog.py` — 从
  `ui/agent-diva-source/agent-diva-providers/src/providers.yaml` 确定性生成
  前端目录的脚本（可重复执行，仓库内即含数据源）。
- `ui/src/components/settings/provider-catalog.ts` — 生成的供应商目录
  （47 家 Agent-Diva 供应商 + vivy 本地 mock，共 48 条）与移植逻辑：
  - `FOLDED_PROVIDER_NAMES`：与 Diva `ProvidersSettings.vue` 的
    `hiddenProviderNames` 完全一致的 20 个折叠 id；
  - `searchProviders` / `splitByFold`：检索按 displayName/name 子串、
    大小写不敏感；搜索时绕过折叠（`more` 恒空），非搜索态按折叠名单拆分；
  - `matchProviderEntry`：用 `(provider, base_url)` 反查目录条目。
- `ui/src/components/settings/provider-catalog.test.ts` — 目录完整性、
  折叠拆分、检索、反查的纯逻辑测试（13 个用例）。
- `ui/src/components/settings/ModelSettingsCard.tsx` — 设置页「Vivy 模型
  配置」卡新实现：左栏供应商列表（搜索框 + 常用供应商 + 「更多供应商」
  折叠行 [MoreHorizontal 图标 + 数量 + chevron] + 当前选中供应商被折叠时
  自动展开），右栏所选供应商的静态模型列表（点击填充默认模型，当前项
  打勾），下方保留原 (Provider / 默认模型 / Base URL) 三输入与显式保存。
- `ui/src/i18n/{zh,en}.ts` — 新增 `settingsModel` 词条块（搜索占位、
  更多供应商、当前徽标、无模型提示、自定义组合提示、无匹配）。

### 修改

- `ui/src/components/settings/SettingsView.tsx` — 模型 Tab 的表单逻辑
  迁入 `ModelSettingsCard`，深链 `?tab=` 行为不变。
- `ui/src/components/chat/MaskAndModelSwitcher.tsx` — 删除本地
  `MODEL_CATALOG`，改用共享目录：按 `(bundle, base_url)` 解析当前厂商
  （如 provider=openai + DeepSeek 网关 → 顶栏显示 "DeepSeek" 并列出其
  模型）；精选模型描述文案（balanced/lighter/…）保留为副标题，未收录
  模型不显示副标题。

## 关键适配（为什么不是逐字移植）

Vivy 后端 `settings.Validate` 只接受 `provider ∈ {"", openai, anthropic,
mock}`，且产品规则要求向原生端点发送原始模型 id、不得自动加网关前缀。
因此目录条目按 Agent-Diva 厂商展示，但选择时映射为 vivy 合法三元组
`(bundle, baseUrl, defaultModel)`：

- `api_type: anthropic` → bundle `anthropic`，其余（diva `api_type:
  openai`）→ bundle `openai`，vivy 本地 → `mock`；
- `default_model` 剥离自家网关前缀（`openrouter/anthropic/claude-sonnet-4`
  → `anthropic/claude-sonnet-4`；`dashscope/qwen-max` → `qwen-max`）；
- diva `custom` 条目的 `custom/default` 占位模型不移植（无推荐模型）；
- 修复 diva 数据 bug：`aionly` 的 `default_api_base` 带全角冒号前缀
  `：https://…`，生成时归一为合法 URL。

## 明确不做

- 模型列表在线刷新（diva 的 `get_provider_models` 运行时拉取）：vivy 无
  provider/model 目录 RPC，目录是静态快照 → 已登记 `docs/TODO.md`
  §0.1 `UI-PROV-RPC`。
- 每供应商 API Key 配置与连接测试向导：密钥只由运行环境管理（产品规
  则，UI 不持有密钥）。
- 自定义供应商的创建/删除（diva 的 custom provider CRUD）。
- 折叠行为的 Playwright e2e：本次以浏览器冒烟覆盖；未新增 e2e 用例。
- 模型 Tab 原有三输入框标签（"Provider/默认模型/Base URL"）等既有硬编码
  中文未迁移 i18n（见 §0.1 既有 `UI-SET-I18N` 条目，避免混入他人条目）。

## 发布说明

无独立发布：随 UI 常规构建发布，`just ci` 已含 ui build，故不单写
`release.md`。
