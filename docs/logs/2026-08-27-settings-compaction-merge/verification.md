# 验证记录（2026-08-27，设置 → 压缩并入通用）

## 执行的命令与结果

- 仓库根目录 `just ci`（fmt-check → vet → go test ./... → headless-compile →
  ui-ci[pnpm install --frozen-lockfile → typecheck → vitest → vite build]）：
  **通过，exit code 0**（两轮：首轮为分区重组改动；次轮加入非法 tab 深链钳制
  修复后复跑仍全绿）。UI 单测 105 passed（15 files），其中
  `diva-preview-data.test.ts`（更新后的压缩排除断言，2 tests）通过；
  vite 生产构建成功（2201 modules）。
- 浏览器真实路径冒烟（针对运行中的拆分布局 `http://127.0.0.1:3015`
  Vite dev server + `:8787` 控制面，临时 Playwright spec 直连 3015，跑完即删）：
  - 「压缩」tab 从设置 Tab 列表消失（tablist 只剩 通用 / 模型 / 工具 /
    Vivy 功能 / 语言 / 通道预览 / 网络预览 / 自进化预览 / 沙箱预览）。
  - 「通用」分区出现「上下文压缩」卡片：`最大 tokens=8192 / 压缩阈值(%)=80 /
    保留最近消息=12`、占用 `6,340 / 8,192 tokens`、按钮「执行压缩预览 /
    恢复预览默认值」。
  - 阈值改为 50 → 压力越过阈值显示「达到压缩阈值」、反馈行「压缩阈值预览已更新。」；
    点击「执行压缩预览」→ 占用变 `2,880 / 8,192 tokens`、反馈「已模拟执行一次
    上下文压缩预览。」；点击「恢复预览默认值」→ 输入回到 8192 / 80 / 12、
    反馈「压缩配置已恢复为预览默认值。」。
  - 深链 `/settings?tab=compaction`：不再选中任何压缩页签，回落到「通用」
    分区并展示「上下文压缩」卡片（修复后）。
  - **冒烟发现并修复的既有缺陷**：`?tab=bogus` 这类非法 tab 在本版本
    `validateSearch` 未实际过滤，`Route.useSearch()` 原样返回非法值，
    `activeTab` 进入非法值导致 Radix Tabs 无匹配、设置页空白（pre-existing，
    与本次删除压缩分区无关，但 `?tab=compaction` 因此落入同一死路）。已在
    `SettingsView` 用 `isSettingsTab(initialTab)` 钳制非法值回落到「通用」，
    冒烟全通过。

## 验证结论

- `just ci` 全绿；用户在 3015 可见行为（tab 删除 / 配置并入通用 / 交互反馈 /
  深链回落）经 Playwright 真实路径全部通过。
- 未验证项：未跑 `just ui-e2e` 全量（既有两条过期规格
  `runtime.spec.ts` / `welcome-wizard.spec.ts` 已知失败，见 `docs/TODO.md`
  §0.1 `UI-E2E-STALE`，与本次改动无关）；本次改动未触碰相关组件。