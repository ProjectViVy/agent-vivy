# 生成参数演示收进「设置 → 通用 → 高级特性」，按模型独立编辑

## 问题（用户反馈，三轮收敛）

1. 最初方案「生成参数演示移到具体 Provider 配置右栏」过于加重 provider 面板，
   用户喊停；
2. 最终方向：生成参数**不放 provider**，也不在模型 Tab 独立显示；收进
   「通用」Tab 的「高级特性」分区；并做模型区分——通过下拉选择具体模型，
   **仅对被选中的模型**可编辑生成参数。

## 改动

- `ui/src/components/settings/GenerationParamsCard.tsx`（新增，通用 Tab 的高级
  特性卡）：
  - 卡头「高级特性」（图标 + 描述：按模型编辑的演示特性，数据只在当前浏览器）；
  - 内部分区「生成参数 + 演示」徽标 + 按模型保存的描述；
  - **模型下拉**：= 已选模型快捷列表 ∪ 当前运行模型（未加入快捷列表也能编辑）；
    默认选中当前运行模型，其次第一个可用模型；
  - 仅对下拉选中的模型显示并编辑温度滑杆（0–2，步进 0.1，数值实时显示）与
    最大 Tokens；「保存演示参数」+ 成功反馈；无可用模型时给空态提示。
- `ui/src/components/settings/SettingsView.tsx`：
  - 「模型」Tab 删除独立「生成参数」卡，只剩「Vivy 模型配置」一张卡；
  - 「通用」Tab 在欢迎向导卡之后挂载 `GenerationParamsCard`；
  - 清理失效的 `demoConfig` / `persistDemo('model')` 分支与 import，工具 Tab
    演示收敛为仅 tools（`loadTools` / `persistTools`）。
- `ui/src/lib/demo-api.ts`：`getDemoGenParams(modelKey)` /
  `saveDemoGenParams(modelKey, params)`——按模型运行三元组键
  `provider/baseUrl/model` 独立存 `vivy.demo.gen-params`（未保存/损坏回默认
  0.7 / 4096，读取不写库）。
- `ui/src/lib/types.ts`：新增 `DemoGenParams`。
- `ui/src/i18n/zh.ts` / `en.ts`：新增 `settings.advancedFeaturesTitle` /
  `advancedFeaturesDescription` / `modelSelect` / `genParamsModelHint` /
  `noModelsForGenParams`；`generationParamsDescription` 改为按模型独立保存口径。

## 未做（显式边界）

- 生成参数仍是演示面（vivy.demo.*，不传真实 Provider），未提升为真实配置。
- 不做可伸缩/折叠交互（用户首条消息提及，最终方向未要求）；「高级特性」卡
  直接平铺。
- `getConfig`/`updateConfig`/`getConfigStatus`/`getRuntimeConfig` 未删除（仍
  作为运行配置演示面保留，本次不再有 UI 使用方）。
- 不触碰根树压缩分区改动（另一条并行 lane 的未提交内容）。
- 既有 `ui/e2e/runtime.spec.ts:84`、`welcome-wizard.spec.ts:33` 过期断言属
  既有问题（`docs/TODO.md` §0.1 `UI-E2E-STALE`），本次未动。