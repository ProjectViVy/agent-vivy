# 验证记录 — 模型设置页视觉梳理 + 生成参数卡主题化

## 自动化

- `cd ui; pnpm typecheck` → 通过（`tsc --noEmit` 无输出）
- `cd ui; pnpm test` → 15 个文件、105 个用例全部通过，含
  `src/i18n/index.test.ts`（zh/en 词典深度结构校验）与
  `src/components/settings/*`（provider-catalog / custom-providers / saved-models）。
- 仓库根 `just ci`（fmt-check + vet + test + headless-compile + ui-ci：
  install --frozen-lockfile + typecheck + test + build）→ 通过（exit 0，含
  `vite build` 正常产出）。
- 会话中另一条并行 lane 在同一共享根树上删除了「人格」演示 Tab（types.ts /
  demo-api.ts / e2e/runtime.spec.ts 以及 SettingsView 的人_格分区）。合并态树
  复验：`pnpm typecheck` / `pnpm test`（105 ✓）/ `pnpm build` 全部通过；实机冒烟
  确认「模型」Tab 正常、人格 Tab 已随另一 lane 移除、无控制台错误、无横向溢出。
  该 lane 的文件保持未 stage，归其提交。

## 实机冒烟（split 对：Vite :3015 + 控制面 :8787，Playwright headless）

页面：`http://127.0.0.1:3015/settings?tab=model`，1440×1000。

- 无控制台错误、无横向溢出（`scrollWidth <= clientWidth`）。
- 渲染断言：
  - 「Vivy 模型配置」「生成参数」卡头渲染正常；
  - 「演示」徽标 1 处、中文「温度」标签 1 处；
  - 「从官方目录同步」按钮 0 处（要求：已移除）。
- 交互断言：
  - 左栏点 OpenRouter → 右栏面板标题由 Mock → OpenRouter（选择联动正常）；
  - 模型分区头部「新增模型」按钮点击 → 弹出「输入模型 id，回车应用」输入框
    （Esc 可收起）；
  - 温度滑杆键盘操作 0.7 → 0.8（ArrowRight）→ 2.0（End），右侧数值文本同步
    （0.7 → 0.8 → 2.0）；
  - 「保存演示参数」点击 → 「已保存到本地」反馈出现。

既有后端契约与数据流未改动：本变更只动 UI 呈现与 i18n 文案，未触碰 settings RPC、
`custom-providers.ts` / `saved-models.ts` / `provider-catalog.ts` 逻辑，故未引入新测试；
复用现有 settings 单测作为回归。

## 未验证

- 深色/粉/初音主题下的实际对比度未截图复核（无图像输入模型）；控件全部使用语义
  Token（bg-accent / bg-primary / muted），理论随主题自适应，界面冒烟在 default 主题
  执行。
- 窄视口（< md）下的两栏布局行为未单独录制（既有 `md:grid-cols-…` 断点未改）。