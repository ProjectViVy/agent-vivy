# 设置 → 语言 可用化（2026-08-27）

## 变更内容

- 设置页「语言」分区从 Agent-Diva 迁移预览升级为真实设置：改为渲染既有但未接线的
  `LanguagePicker`（`ui/src/components/settings/LanguagePicker.tsx`），点击后立即切换
  全局界面语言并持久化到 `localStorage['vivy.language']`，刷新后保持。
- 语言分区不再显示「预览」徽标，成为与「通用 / 模型 / 工具 / Vivy 功能」同级的一等分区；
  深链 `?tab=language` 现在指向真实分区（`SettingsTab` 包含 `'language'`）。
- 删除被替代的伪实现：`DivaSettingsPreview.tsx` 中的 `LanguagePreview` 假预览组件、
  `Languages` 图标导入和 `language` 分支；`diva-preview-data.ts` 的
  `DIVA_PREVIEW_SECTIONS` / `DIVA_ADDITIONAL_SECTIONS` 不再包含 `'language'`，
  `SettingsView` 的 `DIVA_TAB_LABELS` 同步移除该条目。
- 更新 `diva-preview-data.test.ts`：断言 `DIVA_PREVIEW_SECTIONS` 不包含 `'language'`
  且新增的预期列表去掉 `'language'`。
- 新增 e2e 真实路径测试 `ui/e2e/language-setting.spec.ts`：深链语言分区 → 点击 English
  立即切英文 → localStorage / `document.documentElement.lang` 断言 → 刷新持久化 →
  切回简体中文。

## 范围说明

- 只接线路由，没做其他事情：没有新增 i18n 词条（`language.*` 与 `settings.themeSelected`
  字典已存在且双语结构一致）；没有改动 `src/i18n` 本体；其他预览分区（通道 / 网络 /
  压缩 / 自进化 / 沙箱）保持预览状态不变。
- 设置页标签（通用 / 模型 / 工具 / Vivy 功能 / 语言）仍为硬编码中文，与既有模式一致；
  分区「语言」卡片文案跟随当前语言实时翻译。
- 未恢复顺带出现的无关改动；未提交任何内容（未获得提交授权）。