# 删除设置中的「人格」面板

日期：2026-08-27
范围：`ui/`（设置页 persona tab 及其演示 API / i18n / e2e）
归属：Vivy UI 交付

## 结论

设置页「人格」面板已删除：tab、面板内容、演示 API、类型、i18n、e2e 步骤全部移除；`just ci` 全绿，`:3015` 冒烟通过。

## 提交拆分说明（并行 lane 同根树合流）

本次交付在共享根工作树上与另一条并行 lane（模型设置页视觉梳理）同时进行。该 lane 的提交
`30c78b8 ui(settings): 模型设置页视觉梳理 + 生成参数卡主题化` 与我的改动同文件（
`ui/src/components/settings/SettingsView.tsx`、`ui/src/i18n/zh.ts`、`ui/src/i18n/en.ts`）合并，
其提交说明已注明「共享根树上另一条并行 lane 正在删除人格演示 Tab，其 SettingsView 分区改动
与本次同文件合并；该 lane 的 types.ts/demo-api.ts/e2e 改动未随本提交」。

**因此本 log 对应的提交只含我方剩余切片**：
- `ui/src/lib/types.ts`：删除 `PersonaProfile` 接口
- `ui/src/lib/demo-api.ts`：删除 `getPersonaProfile` / `updatePersonaProfile` / `MOCK_PERSONA` / `STORAGE_KEYS.PERSONA`
- `ui/e2e/runtime.spec.ts`：删除「设置 → 人格 tab → 人格配置可见」两步断言

界面分区（`SettingsView.tsx` 的 tab/面板、`i18n` 词条）已在 `30c78b8` 中落地，此处不再重复。

## 变更内容（完整交付清单）

- `ui/src/components/settings/SettingsView.tsx`（已在 `30c78b8` 落地）
  - 删除设置页「人格」tab：`TabsTrigger value="persona"`、`TabsContent value="persona"` 整块内容（名称 / 系统提示词 / 保存人格演示）。
  - `SETTINGS_TAB_VALUES` 去掉 `'persona'`（`?tab=persona` 深链白名单不再接受该值）。
  - 删除 `persona` 状态、`getPersonaProfile()` 加载、`persistDemo('persona')` 分支与对应的 demo API 导入；`demoBusy` / `persistDemo` 类型收缩为 `'model' | 'tools'`。
- `ui/src/lib/demo-api.ts`（本提交）：删除 `getPersonaProfile` / `updatePersonaProfile` / `MOCK_PERSONA` / `STORAGE_KEYS.PERSONA`（仅设置页引用，已成死代码）。侧栏「人格」页使用的 `MOCK_PERSONA_DOCS`、`getPersonaDocument` 等不受影响。
- `ui/src/lib/types.ts`（本提交）：删除仅被设置页使用的 `PersonaProfile` 接口。
- `ui/src/i18n/zh.ts` / `en.ts`（已在 `30c78b8` 落地）：删除 `settings.tabs.persona`、`settings.personaTitle` / `personaDescription` / `savePersonaDemo` / `name` / `systemPrompt`，以及 `demo.personaProfile`（只被已删的 `MOCK_PERSONA` 引用）。
- `ui/e2e/runtime.spec.ts`（本提交）：删除「设置 → 人格 tab → 人格配置可见」两步；保留侧栏「人格」链接到人格页面（该页面仍在）的断言。

## 未做

- 未删除侧栏「人格」页面（`/persona`、`PersonaMemoryView`、`persona.*` i18n、`getPersonaDocument` 等）——用户只要求删除设置中的面板。
- 未触碰 `DIVA_PREVIEW_SECTIONS`（通道/网络/语言/压缩/自进化/沙箱预览区）与模型/工具/Vivy 功能 tab。
- 未移除未使用的 `settings.tabs.*` 其余键（general/model/tools/vivy/preview 标签）——本次只清理与人格面板直接相关的键。