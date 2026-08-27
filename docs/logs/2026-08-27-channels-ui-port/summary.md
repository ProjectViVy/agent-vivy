# 2026-08-27 · 通道配置完整移植（agent-diva → vivy，仅 UI）

## 目标与背景

设置页「通道」分区此前只是 `DivaSettingsPreview` 的**假数据预览**（`DIVA_CHANNELS`
三个静态项 + 只读摘要），无法真正配置。本迭代把 Agent-Diva 设置页的完整通道
配置 UI 移植到 Vivy 前端：卡片视图 + 列表视图 + 内联编辑表单 + 添加/编辑向导 +
平台品牌图标 + 教程弹窗，覆盖全部 7 个 GUI 平台（telegram / discord / feishu /
dingtalk / email / qq / neuro-link），并把「通道」升级为真实设置分区（去掉
「预览」徽标）。

**范围限定「仅 UI」**：不改 Go 后端、不加/改 RPC。因此数据层沿用仓库既定样板
（`custom-providers.ts` / `saved-models.ts`）：localStorage `vivy.ui.channels`
持久化 + 模块缓存 + `useSyncExternalStore`；存储形状对齐 Diva `get_channels`
wire 格式 `Record<通道名, {enabled, ...字段}>`，未来接入后端时直接替换读写层。

## 变更内容

### 新增（`ui/src/components/settings/`）

- `channel-schema.ts` — 移植 `channel-wizard-fields.ts`：7 平台完整凭据字段
  schema（required / secret / group / options / defaults / placeholder / hint）、
  `fieldDefaults`、`fieldsByGroup`、`splitIdList`/`joinIdList`、
  `coerceChannelFieldValue`、`normalizeChannelConfig`、`getRequiredFields`、
  `validateConfig`、`isKnownChannel`。
- `channel-platforms.ts` — 移植平台元数据（displayName / difficulty /
  requiresPublicIP / accessMethod / quickGuideSteps）、`RETIRED_CHANNELS` +
  `isRetiredChannel`（下架：slack / whatsapp / nextcloud_talk / mattermost /
  matrix / irc，延续 Diva 2026-08-18 决策）。
- `channel-store.ts` — `vivy.ui.channels` 本地存储：`getChannels` /
  `saveChannel` / `toggleChannel` / `removeChannel` / `useChannels`；
  读入时应用 `normalizeDiscordConfig`（gateway_url / intents=37377 / 布尔与
  列表默认值，移植自 Diva loadChannels）；`channelStatusFor` /
  `getChannelStatuses` 产出 `{name, enabled, ready, missing_fields, notes}`，
  就绪状态按 schema 必填字段存在性近似计算（后端 `getConfigStatus` 替代）。
- `channel-icons.tsx` — 5 个品牌图标（Telegram / Discord / Feishu / DingTalk /
  QQ）SVG path 转 React 组件 + `PLATFORM_ICONS` / `PLATFORM_DISPLAY_NAMES` /
  `PLATFORM_DESCRIPTIONS`（email → lucide Mail，neuro-link → lucide Globe）。
- `ChannelCard.tsx` / `ChannelCardView.tsx` — 卡片（平台图标 / 就绪徽标 /
  启用状态 / 缺失字段摘要 / 启用·编辑·删除）+ 卡片网格与空态引导。
- `ChannelEditorForm.tsx` — 受控内联表单：text / password（显隐切换）/
  number / select / textarea / string-list / boolean 开关 + 提示文字；
  基础字段直排、高级字段收进 `<details>`；无 schema 通道显示「暂无可用编辑
  字段」并支持未知 extra 键 JSON 编辑。
- `ChannelWizardModal.tsx` — 多步向导（选择平台 → 凭据配置 → 完成）：
  编辑模式预选平台直达凭据步；完成时合并既有配置（保留 `enabled`）；凭据步
  含快速指引面板 + 「查看完整配置教程」。
- `ChannelTutorialModal.tsx` — 教程弹窗（平台概览：接入方式 / 公网 IP /
  难度星级 + react-markdown 渲染内置指南占位，等价 Diva 教程文件缺失回退）。
- `ChannelsSettings.tsx` — 主视图：工具栏（刷新 / 卡片·列表切换 / 添加通道）、
  卡片视图、列表视图（左侧通道列表 + 右侧状态卡 + 内联编辑 + 脏检查保存）；
  删除走 `confirm` + 本地移除（Diva 原为 deleteNotImplemented 桩）。
- `channel-schema.test.ts` / `channel-store.test.ts` — vitest 纯逻辑测试
  （平台覆盖 / defaults / coerce / 列表归一化 / validateConfig /
  localStorage CRUD / Discord 归一化 / 就绪状态 / SSR guard / 坏数据过滤）。

### 修改

- `SettingsView.tsx` — `channels` 进入 `SETTINGS_TAB_VALUES` 显式 Tab
  （标签 `settings.tabs.channels`，无「预览」徽标），挂载 `<ChannelsSettings />`；
  Tab 顺序：通用 / 模型 / 工具 / Vivy 功能 / 语言 / 通道 / 预览分区。
- `diva-preview-data.ts` — 移除 `channels` 预览分区与
  `DivaChannelPreview` / `DIVA_CHANNELS` 假数据。
- `DivaSettingsPreview.tsx` — 删除 `ChannelsPreview` 与 `channels` case。
- `diva-preview-data.test.ts` — 断言更新（预览区不再含 channels）。
- `ui/src/i18n/zh.ts` / `en.ts` — 新增 `channels` 域词典（状态 / 设置 /
  向导 / 卡片视图 / 教程等键，两文件逐键一致，i18n parity 测试自动把关）。
- `docs/TODO.md` §0.1 — 新增 `UI-CHANNELS-BE`：通道配置为纯前端形态，后端
  通道读写与就绪报告未接入。

## 技术决策

- **仅 UI / 本地持久化**：`vivy.ui.channels`（真实功能键，禁用 `vivy.demo.*`），
  wire 形状 = Diva `get_channels`，后端接入时换 `channel-store.ts` 读写层即可。
- **就绪状态 = 本地 schema 校验**：必填齐全 → ready，缺失列于 missing_fields；
  替代服务端 `getConfigStatus` 通道报告，`notes` 恒空（文档已注明差异）。
- **向导省略「测试连接」步**：Diva 源码 `steps` 数组仅 platform/credentials/
  done，「测试」步为当前不可达死代码（`handleWizardTest` 恒返回未实现），
  移植可达三步流程；后端有连接测试能力后再补。
- **删除 = 本地实现**：Diva 原为 `deleteNotImplemented` 桩；「完整通道配置」
  要求删除可用，本地存储删除诚实可行。
- **初始为空态**：不播种假通道；空态卡片引导「添加通道」。
- **标签沿用「通道」**（Diva 源用「频道」）：与 Vivy 现有 Tab 用词一致。

## 明确未做（本轮范围外）

- 未新增后端 RPC（`get_channels` / `update_channel` 等价物）与服务端就绪报告；
  未做真实连接测试；未接 TutorialModal 的外部文档（内置指南占位）。
- 未写 DOM 渲染级组件测试（项目未引入 jsdom / @testing-library，测试保持
  纯逻辑 vitest，与 custom-providers 等样板一致）。