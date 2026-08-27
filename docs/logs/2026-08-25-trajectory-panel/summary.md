# 中控台「审计」→「轨迹」面板（复原 DeepSeek Harness 轨迹设计，演示数据）

Date: 2026-08-25
Status: complete

## What changed

中控台（`/dashboard`）的「审计」功能整体移除，替换为「轨迹」面板
（`TrajectoryPanel`），可见结构与交互尽量复原 DeepSeek Harness
`packages/client/ui-trajectory`（参考 `.workspace/deepseek-harness/upstream/`
工作克隆，未改动上游）。

- **移除**
  - `ui/src/components/audit/AuditPanel.tsx`（整目录删除）。
  - `DIVA_AUDIT_EVENTS` / `DivaAuditTab`（`ui/src/components/settings/diva-preview-data.ts`）
    及其测试断言（`diva-preview-data.test.ts`）。
  - i18n：顶层 `audit.*` 与无引用的 `settings.preview.audit.*` 死键块在
    `ui/src/i18n/zh.ts` / `en.ts` 同步删除；`dashboard.audit*` 键替换为
    `dashboard.trajectory*`。
- **新增** `ui/src/components/trajectory/`
  - `trajectory-demo-data.ts` — 确定性演示数据（24 条记录、3 个回合、8 个
    请求 + 1 次压缩；含 system/user/context/compacted/message/tool/subtool
    全部类型、1 条工具失败、1 条重试），时间戳按固定基线推导，无模块级随机。
  - `trajectory-utils.ts` — 纯投影/格式化工具：三泳道时间轴投影
    （`sequence` 等宽 / `duration` 真实时长压缩空闲，复刻 DSH `timeline.ts`
    语义）、回合/请求起始索引、折叠展示行投影、时长格式化；无 React 依赖。
  - `TrajectoryToolbar.tsx` — 粘性工具栏：实际时长切换、全部回合折叠、
    全部调用折叠、轨迹搜索框（复刻 DSH `TrajectoryToolbar`）。
  - `TrajectoryTimeline.tsx` — Chrome-Network 式 44px 标签栏 + 50px 三泳道
    绘图区：Input/Model/Tools，助理条带 TTFT/解码渐变分段、拖拽选区间、
    悬停 tooltip（KIND · 起止时间 · Total · TTFT · Decoding）、Escape 清除、
    搜索未命中淡化（复刻 DSH `TrajectoryTimeline` + `views.module.css` 布局）。
  - `TrajectoryLedger.tsx` — 两列账本（事件列：回合 `#N` 标签、类型徽标
    图标、`Request #N` 跳转钮、选中/回合竖轨；内容列：`text → result`
    内联预览，错误红色），回合与助理调用链折叠、搜索过滤、时间轴区间外淡化，
    键盘可达（Enter/空格选中）（复刻 DSH `TrajectoryTable` 行结构）。
  - `TrajectoryDetailPanel.tsx` — 右侧详情侧栏：请求级
    （摘要/用量/时序：Status/Provider/Model/工具调用/子调用/错误/重试、
    Token 明细、TTFT/解码/总时长）与记录级（输入/输出/思考 `<pre>`）。
  - `TrajectoryPanel.tsx` — 组合根组件与状态机（折叠、搜索、区间、选中）。
- **接线**：`ui/src/components/demo/DashboardDemoView.tsx` 的「审计」Tab
  改为「轨迹」Tab（`value="trajectory"`），Card 内渲染 `TrajectoryPanel`。
- 新增 `trajectory` i18n 命名空间（zh/en 结构一致，键集：
  工具栏、时间轴、账本、详情、折叠与类型文案）。

## Unchanged

- 概览 / Token 两个 Tab 与 `TokenStatsPanel`、`getDemoDashboard` 快照不变。
- Go 内核、`src/lib/api.ts` / `src/lib/rpc.ts` / `src/lib/store.ts` 零改动，
  无新增后端或 RPC。
- 设置页其它预览分区（channels/network/self-evolution/sandbox）与
  进化页「可审计的进化治理」文案不受影响。

## Scope

UI only（`ui/src`），演示数据，无后端。开发在独立 worktree
`../agent-vivy-trajectory`（分支 `feat/trajectory-panel`）完成并按
`parallel-worktree-isolation` 合回。