# 设置→通用 压缩卡 raw i18n 键修复（UI-I18N-COMPACTION）

## 现象

设置 → 通用 的「上下文压缩」卡片把 `settings.compaction.*` 渲染成原始键
（标题、描述、开关标签、三个表单标签、保存/压缩按钮全部显示键名字面量）；
同时卡内若干文案是硬编码中文（`最大 tokens`、`压缩阈值 (%)`、`保留最近消息`、
`0 = 模型上下文窗口…`、`会话 feed 占用`、`{{percent}}% 占用`、`最近压缩：…`、
`保存中…`、`压缩中…`、占位 `自动`、空态提示），英文界面下也显示中文。

## 根因

键位放错段落，不是键缺失：`zh.ts`/`en.ts` 里确有 `compaction: { … }` 块，
但挂在 `diva` 段下（`diva.compaction`），而组件读的是 `settings.compaction.*`
——键位错配让全部查找 miss，`t()` 回退渲染原始键。全仓库无任何
`diva.compaction` 引用，该块是「压缩配置毕业为真实设置」时挂错位置的死键块。

## 修复

- `ui/src/i18n/zh.ts` / `en.ts`：`compaction` 块移入 `settings` 段
  （zh/en 键位对等迁移），并新增 11 个键：
  `saving`/`compacting`/`maxTokensLabel`/`maxTokensHint`/`autoPlaceholder`/
  `triggerLabel`/`keepRecentLabel`/`feedUsage`/`pressureBadge`/`lastCompaction`/
  `openSessionHint`——消化卡内全部硬编码文案；zh 文案与原硬编码逐字一致
  （中文用户零感知），en 为新翻译。
- `ui/src/components/settings/CompactionSettingsCard.tsx`：10 处硬编码
  中文改 `t()` 调用（含 `pressureBadge`/`lastCompaction` 的插值参数）。
- 新增 `ui/e2e/compaction-setting.spec.ts` 回归规格：zh 默认语言断言卡片
  标题与三个表单标签；断言全页无 `settings.compaction.` 原始键；经
  localStorage 切 English 重载后断言英文标签，且卡内无中文残留、无原始键。

## 明确不做（另开 TODO）

- `DivaSettingsPreview` 通用段（聊天显示/缓存与运行状态/关于 Vivy 及
  「压缩配置已毕业」迁移说明）整体硬编码中文，是预览区既有欠账，
  与本卡无关——登记 TODO 行 `UI-DIVA-PREVIEW-I18N`。
- 设置页 `SettingsView.tsx` 的 `通用` tab 触发器也是硬编码中文（同页相邻
  问题，一并记入欠账行）。
