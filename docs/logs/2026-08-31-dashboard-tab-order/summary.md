# 中控台选项卡重排：Token / 轨迹 / 会话

## 变更内容

中控台（`/dashboard`）选项卡按用户要求调整：

- 默认选中项从「概览」改为「Token」，进入中控台即看 Token 统计。
- 选项卡从左到右依次为：**Token、轨迹、会话**。
- 原第一项「概览」改名为「会话」，内容不变（运行状态 + 近期活动）。
- 页面副标题同步改为「Token 用量、轨迹与会话状态分区展示。」（英文同步）。

## 改动文件

- `ui/src/components/demo/DashboardDemoView.tsx` — `defaultValue="token"`，TabsTrigger/TabsContent 重排为 token → trajectory → overview。
- `ui/src/i18n/zh.ts` — `dashboard.overview`：'概览' → '会话'；副标题重排。
- `ui/src/i18n/en.ts` — `dashboard.overview`：'Overview' → 'Sessions'；副标题重排。

## 明确未做

- 「会话」选项卡内容保持原概览内容（运行状态、近期活动），未接入真实会话列表数据。
- 轨迹面板仍为演示数据（`trajectory-demo-data.ts`），与本次无关。
- 路由、后端 RPC、数据结构均无变化。
