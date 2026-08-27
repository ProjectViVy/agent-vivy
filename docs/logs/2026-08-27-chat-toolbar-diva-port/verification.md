# Verification

Date: 2026-08-27

范围：聊天框上方功能栏移植（Agent-DIVA → Vivy，仅 UI）。改动位于
`ui/src/components/chat/ChatInput.tsx`、`ui/src/lib/store.ts`、
`ui/src/routes/_layout.tsx`、`ui/src/i18n/zh.ts`、`ui/src/i18n/en.ts`。

## 门禁：`just ci`（仓库根目录）

结果 **EXIT=0**：Go fmt-check / vet / test / headless-compile 全部通过
（internal/app 3.229s 等，余者 cached）；UI `tsc --noEmit` 通过；
vitest **15 files / 105 tests passed**（含 i18n 词条平价 `index.test.ts`
9 例 —— zh/en 结构一致的硬校验，以及 `store.test.ts` 4 例）；
`vite build` 成功（仅既存 >500 kB chunk 体积提示）。UI 侧无新增
组件级单测（ui 无 @testing-library 基建，小改动不为此前置搭建自动化，
以 typecheck + vitest + 真实路径冒烟覆盖）。

## 真实路径冒烟：http://127.0.0.1:3015（split Vite :3015 + 真实控制面 :8787）

前置：:3015（PID 9276）与 :8787（PID 22900）已在运行（既有 `just dev`
拆分对），冒烟直接用当前 Vite dev server（源码即改即生效）。用
`@playwright/test`（chromium-1234，headless）驱动，先置
`vivy.ui.welcome.completed=1` 跳过首次向导。随机脚本销毁，未入库。

结果 **22/22 PASSED**（`SMOKE RESULT: PASSED`）：

- 工具栏 8 项在 DOM 中按 DIVA 顺序齐备：模式触发（智能体模式、
  附件、思考模式、AutoDream、打开伙伴、权限触发（智能）、历史、
  审批中心；
- 「画图」「智能」按钮确认已移除；
- 模式下拉：打开后菜单项为「智能体模式（直接执行任务）/ 计划模式
  （先规划再执行）/ 询问模式（只读分析模式）」，选「计划模式」后
  触发器文字更新为「计划模式」；
- 思考下拉：菜单项 自动 / 开启 / 关闭；
- 权限下拉：菜单项「谨慎（所有操作均需确认）/ 智能（低风险自动放行）/
  信任（仅高风险需确认）」，选「谨慎」后触发器更新；
- 三个 stub：点附件 → 「附件功能暂未接入」，点 AutoDream →
  「AutoDream 暂未接入」，点伙伴 → 「桌面伙伴暂未接入」（底部
  `aria-live` 提示条，约 1.8s 消失）；
- 历史（时钟）→ 右侧「会话」Sheet 打开；审批中心 → 右侧
  「审批中心」Sheet 打开；
- 双语：置 `vivy.language=en` 重载后按钮名称为 Agent mode /
  Thinking mode / History / Approvals / Smart，`documentElement.lang=en`；
  恢复 zh 正常。

冒烟中发现并修正一处仅测试脚本问题：首跑 i18n 步骤在 reload 后
未等待聊天区重挂载（2.2s 固定等待早于渲染），断言误报；改为
`waitFor` textbox 后再断言即通过（产品功能本身无问题，DOM probe
确认 lang=en 且工具栏按钮英文文案齐全）。

## 跳过说明

- 未单独跑 `go test`（`just ci` 已覆盖；本次无 Go 侧改动）。
- 未跑 `just ui-e2e`（既有 e2e 两条规格已因过期文案失败，见
  `docs/TODO.md` §0.1 `UI-E2E-STALE`，与本迭代无关）；本迭代以
  真实路径 Playwright 冒烟代替。若需纳入回归，可在 UI-E2E-STALE
  修复后补跑。