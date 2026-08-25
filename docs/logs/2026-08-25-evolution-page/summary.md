# Evolution page (进化页)

Date: 2026-08-25
Status: complete

## What changed

侧边栏「进化」入口从 pending 占位（点击弹「暂未接入」提示、无路由）变为可进入的
`/evolution` 页面。页面参考 agent-diva `EvolutionView.vue` 的治理结构，按 Vivy
演示页既有模式（`demo-api.ts` + `vivy.demo.*` localStorage）实现，与技能页共享同一份
存储（单一权威来源）。

三个 Tab，均为 master-detail 双栏（移动端单栏 + 返回）：

- **Skill**：仅列 `evolution_managed` 的 skill。详情含权威 Markdown（查看/编辑保存，
  CAS base-hash 冲突拒绝）、启用/停用、硬删（AlertDialog 确认，仅 `can_hard_delete`）、
  历史快照列表与逐版本全文预览。
- **待审**：请求列表（状态徽章：待审/已接受/已拒绝/已失效）+ 详情（提案全文、基准哈希、
  Evidence/Attestation 面板）。「接受」按 base_hash 与当前权威头比对：不一致置 stale 并
  拒绝；一致则把提案写入权威头、保留历史快照、置 `evolution_managed`。skill 不存在时
  接受会创建 home skill。「拒绝」带确认框，仅 pending 可操作。含「新建请求」表单
  （slug/标题/原因/声明/提案 Markdown，前端必填校验）。
- **AutoDream**：运行列表（状态徽章）+ 详情（触发方式/阶段/起止时间/尝试次数/失败原因/
  输入摘要、进度事件时间线、提案跳转回待审 Tab 并选中）。

配套改动：

- `ui/src/routes/_layout.evolution.tsx`：新路由（persona 式标题头 + DemoBanner）。
- `ConversationSidebar.tsx`：删除 pending 分支、notice 机制与 `Badge` 死代码；
  `nav.evolutionPending` / `nav.evolutionUnavailable` 词条随之删除（zh/en 同步）。
- `types.ts`：新增 AutoDream 域类型（run/事件/输入摘要/编排阶段，与 agent-diva
  wire 类型同构）。
- `demo-api.ts`：新增 `vivy.demo.skill-docs`（权威文档+历史）与 `vivy.demo.autodream`
  存储；`getSkillDocument` 从读模块常量改为读本地存储（治理写入才能在文档视图生效）；
  新增 accept/reject/updateSkillDocument/setSkillEnabled/deleteSkill/history 读写函数；
  种子数据含 2 个进化管理 home skill、3 条混合状态请求、3 条 AutoDream 运行。
- **修复播种共享 bug**：`readSkillStore`/`readSkillRequests`/`readSkillDocStore`/
  `readAutoDreamStore` 播种时原先返回模块级 MOCK 数组本体，治理操作原地修改会污染
  模块种子（跨测试/清缓存重播种可见）。现统一经 `seedStore` 写入并返回副本。
- `SkillsView.tsx`：请求状态徽章从原始枚举改为 `skills.status.*` 本地化（演示请求
  种子后技能页同样可见）。
- i18n：顶层 `evolution` 域 + `skills.status` + `demo.evolution` 演示文案，zh/en 同步。
- 新测试 `demo-api.evolution.test.ts`：播种、接受写头+历史、stale 判定、重复决策拒绝、
  CAS 冲突、硬删边界（内置不可删）。

## Explicitly not done

- 内核 Evolution/AutoDream 真实能力（MEM-1 维持 DEFERRED；本页不接 `api.ts` RPC，
  不登记新 RPC_METHODS）。
- agent-diva 页的搜索框（演示数据量级下无辨识价值）、AutoDream 实时输出轮询
  （演示数据为终态记录，轮询即伪操作）、CodeMirror 编辑器（用 Textarea）。
- `diva.evolution.*` 设置页预览文案不动（属 Agent-Diva 迁移预览分区）。

## Scope

- `ui/src/components/evolution/EvolutionView.tsx`（新增）
- `ui/src/hooks/useEvolution.ts`（新增）
- `ui/src/lib/demo-api.ts`、`ui/src/lib/types.ts`
- `ui/src/routes/_layout.evolution.tsx`（新增）、`ui/src/routeTree.gen.ts`（生成）
- `ui/src/components/chat/ConversationSidebar.tsx`、`ui/src/components/skills/SkillsView.tsx`
- `ui/src/i18n/zh.ts`、`ui/src/i18n/en.ts`
- `ui/src/lib/demo-api.evolution.test.ts`（新增）
- 参考（未改动）：`C:\Users\Administrator\Desktop\morediva\agent-diva\agent-diva-gui\src\components\EvolutionView.vue`
