# 2026-08-25 侧边栏 Skill 选项"遮罩"BUG 修复 + 进化占位入口

## 问题

左侧栏存在两个指向同一路由 `/skills` 的导航项：

- 「进化」（Vivy 分组，`nav.evolution`）
- 「Skill」（工具管理分组，`nav.skill`）

进入 `/skills` 页面时，TanStack Router 同时给两个 `<a>` 打上 `active`，
「进化」与「Skill」同时高亮（`bg-sidebar-accent`）。用户点击「进化」（另一个
选项）后，「Skill」选项也被高亮背景覆盖，即「点击其他选项自动遮罩 SKILL 选项」。

根因：`ui/src/components/chat/ConversationSidebar.tsx` 中 `VIVY_ITEMS` 与
`TOOL_ITEMS` 都登记了 `to: '/skills'`，而路由只有唯一一个 `/skills`
（SkillsView 技能管理页），必然同时命中高亮判定（`pathname.startsWith(item.to)`）。

## 变更

- `ui/src/components/chat/ConversationSidebar.tsx`：
  - 「Skill」保持为 `/skills` 唯一可导航入口（工具管理分组）。
  - 「进化」以**占位入口**形式加回 Vivy 分组：渲染为非导航 `<button>`（不再
    是 `<Link>`），带「待实现」`Badge`；点击按应用既有「暂未接入」惯例显示
    提示（1.8s 自动消失），不会跳转、不会命中高亮。为此组件新增 `pending`
    标记与 `showNotice` 本地提示（与 `ChatInput` 的未接入按钮同一模式）。
  - 引入 `Badge` 组件；恢复 `Dna` 图标导入。
- `ui/src/i18n/zh.ts` / `ui/src/i18n/en.ts`：恢复 `nav.evolution`，新增
  `nav.evolutionPending`（待实现 / Planned）、`nav.evolutionUnavailable`
  （进化功能暂未接入 / Evolution is not implemented yet）。

## 设计说明

产品侧 Evolution/AutoDream 能力在 `docs/TODO.md` §0.1 仍为 DEFERRED，没有独立
路由。因此「进化」不做成跳转入口（跳 `/skills` 会重新引入双重高亮，跳不存在
的路由会 404），而是保留在导航里、用「待实现」标签标明状态，点击给出与
ChatInput 未接入按钮一致的提示。

## 明确未做

- 移动端抽屉点击「当前已所在路由的导航项」时抽屉不会关闭（`pathname` 未变化，
  `_layout.tsx` 的 `useEffect([pathname])` 不触发）。独立小边界场景，不在本次
  报告范围内，未改动。
- 未新建 `/evolution` 路由或占位页面——Evolution 能力未实现，等实现时再补。
