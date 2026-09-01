# UI-AUDIT-DASHBOARD-LIVE — Dashboard 概览接真实 RPC，删除演示快照

## 变更

`/dashboard` 的 Overview Tab 不再读 `vivy.demo.dashboard` localStorage
演示快照，改为并行调真实 RPC：

- 会话数 → `session/list`（`sessions.length`）
- 活跃运行 → `background/list`（status 非终结态
  `completed/failed/cancelled` 的运行数；审查行指定的 background RPC，
  内核无"列出全部运行"端点，后台注册表即运行清单）
- 待处理 Review → `review/list`（`status: 'pending'` 由后端过滤，
  覆盖 approval + question 两类）

错误态沿用 `DemoLoadError`（共享错误横幅，Token Tab 同款）+ 重试；
加载中保持骨架屏。**近期活动卡整卡删除**（审查行裁定：活动项无现有
端点则删除——内核没有通用活动流 RPC，演示活动项是编造数据）。

## 删除的演示面

- `demo-api.ts`：`getDemoDashboard`、`DEFAULT_DASHBOARD`、
  `STORAGE_KEYS.DASHBOARD`（`vivy.demo.dashboard`）
- `types.ts`：`DemoDashboardSnapshot`
- i18n en/zh：`dashboard.activityTitle/activityDesc`、
  `demo.dashboard.*`（活动项文案块）
- 组件更名以正名：`components/demo/DashboardDemoView.tsx` →
  `components/dashboard/DashboardView.tsx`（路由 `_layout.dashboard`
  同步改 import）；组件内其余演示命名（`DemoLoadError`、
  `TokenStatsPanel` 位于 `components/demo/`）不动——它们是真实面板
  在用的共享件，不属本行。

## 范围外（发现并另立新行）

- Trajectory Tab 的 `TrajectoryPanel` 仍是纯演示数据
  （`trajectory-demo-data.ts`，组件注释自述"无后端"）。审查行只规定了
  overview 数字与活动项，轨迹面板是独立大面——另立 TODO 行
  UI-TRAJECTORY-DEMO 跟踪，不在本切片扩权。
- Token Tab 已是真实 `stats/tokens`，无需改动。

## 本来就真实的部分

`TokenStatsPanel`（`stats/tokens` 周期快照、模型分布、趋势、导出）
与路由结构保持原样。
