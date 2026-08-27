# 设置 → 压缩分区并入通用（2026-08-27）

## 变更内容

- 删除设置页「压缩」独立分区：`diva-preview-data.ts` 的
  `DIVA_PREVIEW_SECTIONS` / `DIVA_ADDITIONAL_SECTIONS` 不再包含 `'compaction'`，
  `SettingsView.tsx` 的 `DIVA_TAB_LABELS` 同步移除 `compaction` 条目，顶部 Tab
  列表不再出现带「预览」徽标的「压缩」页签。
- 压缩配置移动到「通用」分区：`DivaSettingsPreview.tsx` 的 `GeneralPreview`
  新增「上下文压缩」卡片（预算进度条 + 最大 tokens / 压缩阈值 / 保留最近消息
  三个输入 + 执行压缩预览 / 恢复预览默认值），弹出位置在「聊天显示」与
  「缓存与运行状态」之间；通用分区描述同步提及「上下文压缩」。
- 删除被替代的伪实现：`CompactionPreview` 独立预览组件、`Minimize2` 图标导入
  和 `DivaSettingsPreview` 的 `case 'compaction'` 分支。
- 更新 `diva-preview-data.test.ts`：断言 `DIVA_PREVIEW_SECTIONS` 不包含
  `'compaction'`，新增预期列表去掉 `'compaction'`。
- 深链 `?tab=compaction` 因 `isSettingsTab` 白名单变化静默回落到默认「通用」
  分区（路由本就对非法值静默丢弃，无需改动）。
- 顺带修复非法 tab 深链的既有缺口：`SettingsView` 初始化与 initialTab 变化
  effect 现在用 `isSettingsTab(initialTab)` 钳制，非法值一律落到「通用」分区。
  此前 `?tab=bogus` 等非法值会原样进入 `activeTab`，导致 Radix Tabs 无匹配值、
  设置页空白（`validateSearch` 在本版本实际未过滤，debug 确认
  `Route.useSearch()` 原样返回非法值）；本次把 `?tab=compaction` 划入非法
  值路径，故在同一交付里一并修复，使旧深链落到通用页而非空白页。

## 范围说明

- 只做分区重组，未动其他内容：压缩预览的文案、默认值、交互行为原样保留，
  仅改变所在位置与卡片合并方式（原两卡「预算状态 / 压缩配置」合并为一张
  「上下文压缩」卡）。
- 未改动 `src/i18n`：`settings.tabs.compaction` 与 `diva.compaction` 字典条目
  本来就没有引用（预览组件均为硬编码中文），与既有「语言分区升级」迭代的模式
  保持一致，本次不动。
- 其他预览分区（通道 / 网络 / 自进化 / 沙箱）保持预览状态不变。
- 未提交任何内容（未获得提交授权）。