# 2026-08-25 设置页布局与说明一致性整理

## 变更

设置页（`ui/src/components/settings/`）按 oil-frontend 规范（视觉工程 + 信息与动作）做了一致性整理：

1. **保留「通用与关于」**：上一轮删除该预览分区后按用户要求完整回滚（`DivaSettingsPreview.tsx` 的 `GeneralPreview`、`SettingsView.tsx` 的挂载点、zh/en 词典的 `diva.general` 文案块全部恢复）。
2. **统一卡片头部结构**：`SettingsView.tsx`「Vivy 功能」页的 Run Inspector 卡原用「图标在标题旁」的一次性写法，改为与 ThemePicker / 生命周期卡一致的「图标在上 + 标题 + 说明」标准结构。
3. **删除无职责的 DemoNote**：通用页的 DemoNote 声称「修改只保存到 vivy.demo.* localStorage」，但该页没有任何内容写 vivy.demo.*（ThemePicker 是真实持久化，预览区有自己的 Agent-Diva 声明），属于信息与动作规范要求删除的无职责文案。
4. **预览标签加「预览」标记**：六个 Agent-Diva 迁移预览标签页（通道/网络/语言/压缩/自进化/沙箱）在标签栏与真实标签无法区分，统一加「预览」小标记，让真实/预览边界在标签栏即可辨认。
5. **补齐两张缺失说明的预览卡**：网络页「当前预览摘要」、压缩页「压缩配置」补上与同页其他卡一致的说明（说明预览数据作用范围），消除「有的卡有说明、有的没有」的不一致。

## 阻断修复（并行工作流遗留）

工作区存在另一并行 i18n/masks 工作流的未提交改动，导致应用无法挂载、`just ci` 无法通过：

- `mask-catalog` 改为函数式导出（`maskOptions()`），但 `MaskAndModelSwitcher.tsx`、`MaskManagementView.tsx` 仍引用 `MASK_OPTIONS`（运行时未定义 → 应用白屏）。
- `LifecycleView.tsx` 模块级 `GenerationSelect` 使用 `t` 但作用域内未定义（类型错误 + 运行时 ReferenceError）。

这三处为机械性、无歧义修复，属解锁验证的必要工作，已一并修复并记录。

## 明确未做

- **未接线 i18n**：`DivaSettingsPreview` 仍硬编码中文，`diva.*` 词典（zh/en）无组件消费；`LanguagePicker.tsx` 已建未挂载。这些属于并行 i18n 工作流的进行中状态，本次不触碰，已记入 `docs/TODO.md` §0.1（UI-SET-I18N）。
- 未改动 ThemePicker / LanguagePicker（并行流文件）。
- 未改动 masks / lifecycle 的业务逻辑，仅修复导出与作用域。
