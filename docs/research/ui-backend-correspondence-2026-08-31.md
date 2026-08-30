# UI ↔ 后端对应关系综合审查（排除通道）

日期：2026-08-31  
审查分支：`feat/ui-backend-correspondence-audit`  
范围：Vivy 日常 Web UI（`ui/src`）与当前 JSON-RPC 控制面、runtime、产品契约的双向对照。  
明确排除：通道；Studio 壳；后端尚未提供 RPC/数据模型的演示功能；生产 Journal 与 `data/`。

## 结论摘要

主链路不是整体脱离后端：会话/消息/运行与 SSE 回放、权限预设、Review Center 的基础队列与决定、Provider/网络/MCP 设置、Token 统计、上下文读数、后台运行和 child-run 均已找到真实 RPC 对应。问题集中在以下几类：

1. UI 暴露了后端已有能力但没有传递参数（聊天 Plan 模式）。
2. UI 用本地演示层遮住了后端已有只读能力（Skills、Dashboard 概览）。
3. UI 把已有物种侧兼容 RPC 当成日常产品权威（Lifecycle），违反 Studio 边界。
4. UI 没有把后端已有的 Review/event 细节放到运行检查和详情中，且忙状态范围过大。

共记录 10 项：P1 6 项、P2 4 项；没有发现本次范围内的 P0 安全/数据破坏项。修复建议按 P1 → P2 排序，详见“建议执行顺序”。

## 证据基线

- UI RPC 类型与调用：`ui/src/lib/api.ts`、`ui/src/lib/store.ts`。
- 控制面分派与 wire DTO：`internal/rpc/control.go`。
- 运行/压缩语义：`internal/runtime/compaction_service.go`、`internal/runtime/compaction_middleware.go`。
- 产品边界：`docs/architecture/VIVY-STUDIO.md`、`docs/architecture/VIVY-GATEWAY-AND-STUDIO.md`。
- Review 合同：`docs/architecture/hitl-review-center.md`。
- V0 UI/日志要求：`docs/research/prd-agent-vivy-v0.md`。

## 发现清单

### P1 — UI-AUDIT-CHAT-MODE：Plan 模式选择不会到达后端

**证据**

- `ui/src/lib/api.ts:20-21,200-201` 明确定义后端 `RunMode` 只有 `normal | plan`，`preflight` 和 `startTurn` 都接受 mode。
- `ui/src/components/chat/ChatInput.tsx:20,29,37,60,121,159-170` 定义并切换 `agent/plan/ask`，但 `onSend` 只传一个 `content`，发送时没有传 `execMode`。
- `ui/src/components/chat/ChatView.tsx:44,55,89` 固定以 `'normal'` 调 `preflight`，随后以默认参数调用 `startRun`；没有把菜单选择映射到后端 mode。

**影响**

用户选中 Plan 后，预检与实际运行仍是 normal；界面状态与 `preflight.mode`、运行记录不一致。`ask`、thinking 等没有对应后端 mode 的选项不纳入本项，只登记为范围外。

**应改**

让 `ChatInput` 的回调携带后端可表达的 mode，`ChatView` 同时把它传给 `preflight` 和 `startRun`；菜单只把有后端契约的选项标成可执行。同步更新现有 `docs/TODO.md` 的 `UI-CHAT-TOOLBAR` 说明。

### P1 — UI-AUDIT-SKILLS-LIVE：Skills 列表/详情仍使用 localStorage 演示层

**证据**

- `ui/src/routes/_layout.skills.tsx:3-4` 将整个技能页包在 `DemoBanner` 中。
- `ui/src/hooks/useSkills.ts:6-7,11,22,35` 从 `@/lib/demo-api` 读取列表和文档，使用 `SkillDto/SkillDocument` 的 `slug/enabled/source/markdown` 等演示字段。
- `ui/src/lib/api.ts:373-391` 已有真实 `SkillSummary` / `SkillView` 以及 `skills/list`、`skills/get`。
- `internal/rpc/control.go:41-42,368,480-482,792-825` 已注册并实现只读技能目录。

**影响**

真实 `skills_root` 的名称、描述、hash、warnings、相对路径和 supporting files 不会显示；浏览器 localStorage 中的种子数据可能与后端安装内容完全不同。列表/详情虽有后端能力，却不可见。

**应改**

将列表/详情 hook 改为 `api.listSkills/getSkill`，以 `name` 作为身份并展示后端已有字段；移除该真实页上的 DemoBanner。技能变更请求/启用删除等没有当前 RPC 的演示 tab 保持明确 Demo 或移除，不应冒充已接后端。

### P1 — UI-AUDIT-DASHBOARD-LIVE：Dashboard 概览显示固定演示数字

**证据**

- `ui/src/routes/_layout.dashboard.tsx:3` 直接渲染 `DashboardDemoView`，没有 `DemoBanner`。
- `ui/src/components/demo/DashboardDemoView.tsx:5,18,52-60` 调 `getDemoDashboard()`，把 `sessionCount/activeRuns/pendingReviews` 渲染成概览状态。
- `ui/src/lib/demo-api.ts:1622-1624` 的默认值是固定的 `12/2/1`，并写入 localStorage。
- 后端已提供 `session/list`、`background/list`、`review/list`，且控制面 capability 在 `internal/rpc/control.go:363-368`；Token 子页在 `DashboardDemoView.tsx:96` 已改为真实 `stats/tokens`，这是正确做法。

**影响**

用户看到的是“运行状态”语义，却可能永远是本地种子数；多窗口、重启或真实会话都不会反映。页面名称和位置也没有提示这些数字是演示数据。

**应改**

用 store/API 的真实 session、background、review 数据计算三项计数；“最近活动”没有现有后端事件查询聚合接口，应删除或只显示已有可证明来源。保留真实 Token 子页；轨迹 tab 因后端没有对应端点，按范围外处理并继续显式标 Demo。

### P1 — UI-AUDIT-LIFECYCLE-HOME：物种 UI 暴露 Generation/Eval/Promote 写权限

**证据**

- `ui/src/routes/_layout.lifecycle.tsx:2-3` 暴露 `/lifecycle`。
- `ui/src/components/lifecycle/LifecycleView.tsx:17-26` 通过 store 调 `generations/create/reject`、`evals/start/record`、`promotions/promote`，并提供写入表单。
- `ui/src/components/settings/SettingsView.tsx:204` 在日常设置页提供“打开生命周期”入口。
- 正本 `docs/architecture/VIVY-STUDIO.md:19-21,52,127-145,258,354-361` 明确 Studio 独立拥有开发/评测/发布生命周期，物种只保留只读 inspect；物种侧这些表是“错误的家”。`docs/architecture/VIVY-GATEWAY-AND-STUDIO.md:224-225,412-417,507-512` 同样冻结 species-side authority。

**影响**

后端 RPC 虽然仍存在兼容实现，但日常 Vivy UI 把它们呈现为产品权威，混淆住户产品与 Studio，用户可以在错误的边界发起评测/晋升。

**应改**

从日常 Vivy 移除生命周期写页面和 Settings 卡；若需要诊断，只保留 `species/inspect` 的只读视图。Generation/Eval/Promotion 的写操作与账本入口迁移到 Studio，不扩展物种侧产品语义。

### P1 — UI-AUDIT-REVIEW-INSPECTOR：运行检查未提供 Review inline surface

**证据**

- `docs/architecture/hitl-review-center.md:31-38` 要求决定/过期/stale 可由 run inspector 回放，并要求 inspector 使用与 Review Center 相同的 `renderReviewCard` 做 inline decision。
- `docs/TODO.md` 的 HITL-04 已记为“Review Center + run inspector Review tab”完成。
- 当前 `ui/src/components/chat/RunInspector.tsx:27-31` 只有 current/background/children 三个 tab；current tab 只显示事件列表，未导入 `ReviewItem`、未显示 Review tab，也没有 `review/respond` 控件。
- 后端 `review/list/get/respond` 已在 `ui/src/lib/api.ts:73,218-220` 和 `internal/rpc/control.go:1054-1162` 提供。

**影响**

用户在运行上下文中看不到挂起的 approval/question，也不能从运行检查内完成决定；需要离开当前运行打开 Review Center，且实现与已登记的 HITL-04 状态不一致。

**应改**

在 inspector 增加 Review tab/inline card，复用 Review Center 的 `ReviewItem` renderer 和分开的 approval/question controls；按 run/session 过滤并在事件到达时刷新，保持后端 first-writer-wins。

### P1 — UI-AUDIT-RUN-DETAIL：结构化事件 payload 只放在 title，不满足可读日志

**证据**

- V0 正本 `docs/research/prd-agent-vivy-v0.md:90-96,301-302` 把 persisted events、结构化错误和无需开发工具即可阅读的 run-detail/event-log 定为产品要求。
- `ui/src/lib/api.ts:50` 的 `RunLogEvent` 明确含 `seq/type/created_at/payload_version/payload`。
- `ui/src/components/chat/RunInspector.tsx:28` 只把 `event.type` 作为可见文本，把 `JSON.stringify(event.payload)` 放在 HTML `title`；没有展开详情、时间、payload version、错误分类或安全的字段摘要。

**影响**

桌面鼠标悬停才能看到粗糙 JSON，窄屏/键盘/触摸不可读；工具调用、策略、重试、压缩和失败原因等后端已记录的信息无法作为用户可读审计轨迹。

**应改**

提供键盘可达的事件详情/折叠面板，显示 seq、时间、类型、payload version 和按事件类型渲染的安全摘要；敏感参数沿用后端脱敏，不把原始 secret 放入 UI。

### P2 — UI-AUDIT-REVIEW-FIELDS：Review 详情丢失后端已有审计字段

**证据**

- `internal/rpc/control.go:314-342` 的 `reviewResult` 含 source、actor、created/expires/decided 时间、precondition hash、decision/stale/error reason 等；`ui/src/lib/api.ts:73` 同步了这些字段。
- `ui/src/components/approvals/ApprovalsView.tsx:83-119` 只渲染 run、action、target、effect、reversibility、scope、trust、prompt、preview、risk、redacted arguments，未渲染 source/actor/timestamps/expiry/precondition/terminal reason。
- `docs/architecture/hitl-review-center.md:7-12,23-32` 将这些字段和 expiry/stale 生命周期列为 ReviewItem 合同的一部分。

**影响**

expired、stale、denied、cancelled 等状态没有时间线和原因，用户无法判断提案来自谁、是否过期、哪个前置条件失效，审计能力被削弱。

**应改**

补齐 source/actor、创建/过期/决定时间、precondition hash、decision/stale/error reason 的本地化展示；对 pending 与 terminal 状态使用不同的动作和说明。

### P2 — UI-AUDIT-COMPACTION-BUSY：立即压缩按钮未反映后端忙状态

**证据**

- `ui/src/components/settings/CompactionSettingsCard.tsx:66-85,150-153` 的“立即压缩”只在没有 active session 或自身 `compacting` 时禁用，没有检查当前/后台 run。
- `internal/runtime/compaction_service.go:110-125` 在任一 active/pending run 时返回 `ErrCompactionBusy`；`internal/rpc/control.go:768-789` 将其作为 conflict 返回。

**影响**

运行中用户可以点击看似可用的操作，随后收到后端 409；即使是另一个 session 的后台 run，也没有提前解释为什么失败。

**应改**

从 store 暴露 active/background busy 语义，运行期间禁用并说明“运行内会自动压缩”；终止后刷新上下文再启用。后端仍是最终权威。

### P2 — UI-AUDIT-REVIEW-BUSY-SCOPE：单条 Review 响应锁住整个队列

**证据**

- 后端 `review/respond` 是针对单个 `review_id` 的条件响应（`internal/rpc/control.go:1100-1162`）。
- store 只记录一个 `reviewBusyId`（`ui/src/lib/store.ts:387-398`）；但 `ui/src/components/approvals/ApprovalsView.tsx:57,70,122,126-132,150` 用 `busyId !== null` 禁用所有列表项、详情动作和刷新。

**影响**

处理一条 approval 时，其他 session 的 question/review 不能查看或处理，忙状态范围大于后端请求范围。

**应改**

只锁定当前 `reviewBusyId` 的行和动作；允许浏览/选择其他 review，必要时仅阻止同一条重复提交。刷新可在请求完成后继续。

### P2 — UI-AUDIT-REVIEW-NAV：完整 Review 路由不在主导航

**证据**

- `ui/src/routes/_layout.approvals.tsx:2-3` 存在完整 `/approvals` 主区域页面。
- `ui/src/components/chat/ConversationSidebar.tsx:4-9` 的 `NAV_ITEMS/VIVY_ITEMS/TOOL_ITEMS` 没有 `/approvals`；`ui/src/components/chat/ChatInput.tsx:241-242` 只有聊天框里的 shield 按钮打开 sheet。
- Review 合同 `docs/architecture/hitl-review-center.md:36-38` 要求 Review Center 是跨 session 的 full main-area surface。

**影响**

用户必须先进入聊天才能发现跨 session 队列，直接主区域路由虽存在却没有稳定入口；在设置、Dashboard 或窄屏布局中不易回到 Review Center。

**应改**

把 Review Center 加入主导航或提供全局明显入口；保留现有 sheet 作为就地处理，不删除完整路由。

## 已核实对应、因此不列为缺陷

- 会话创建/列表/重命名/删除、消息列表、运行启动/取消/重连回放：`ui/src/lib/store.ts` 使用 `session/*`、`turn/*`、`run/*` RPC。
- 权限预设已传到 `session/set_permission`，并在运行中锁定；Provider 注册表、网络工具、Sandbox、Compaction 配置和 MCP 均走对应 settings RPC，且尊重 read-only/frozen。
- Review Center 主队列已走 `review/list/get/respond`，approval 与 question 控件分开；本报告只指出详情字段、inline inspector、忙范围和导航缺口。
- Token 统计页已走 `stats/tokens`（`DashboardDemoView.tsx:96`），不把该页重新算成演示错配。
- Todo 进度使用真实 `session/todos` 且只读；后端没有人为 mutation，因此不把“不能编辑”算入本次范围。
- 后台 run attach、child-run 基础列表/打开/等待/取消已对应 RPC；树形可视化是已有 `UI-TREE` deferred 项，不重复登记。

## 明确排除（后端没有，不纳入本次问题清单）

- 通道配置与就绪报告：`UI-CHANNELS-BE`，用户已明确要求排除通道。
- Evolution/AutoDream、Persona/Memory、Notebook、Cron、trajectory、工具规则演示、生成参数、Diva preview：当前没有相应 Vivy 控制面端点；沿用 `UI-EVO`、`UI-TRAJ` 等既有 TODO，不把“演示层”本身当成后端错配。
- 聊天 `ask`/thinking、附件、AutoDream、桌面伙伴、语音，以及消息 edit/rewind/fork：后端没有对应当前 RPC；`UI-CHAT-TOOLBAR`、`UI-CHAT-ACT` 已登记。
- 技能 change-request、启用/删除/编辑：本次只审查后端已有的 list/get；请求/写操作没有后端端点，保持 Demo 标识或另立能力提案。
- `http_request` 独立设置、todo 人工 mutation、goal：已有 TODO 明确记录为后端能力/产品提案缺口，按用户要求不纳入。

## 建议执行顺序

1. P1：先修 Plan mode 参数链；同时从日常 Vivy 移除/降级 Lifecycle 写入口。
2. P1：Skills 列表/详情和 Dashboard 概览切到真实 RPC；Dashboard 活动项没有端点则删除。
3. P1：补 Run Inspector 的 Review inline 与可读事件详情，恢复 HITL-04/日志正本要求。
4. P2：补 Review 审计字段、单项 busy scope、主导航入口，再修 Compaction busy 预判。

## 验证状态

- `pnpm install --frozen-lockfile`：通过（隔离 worktree）。
- `pnpm build`：通过。
- `just ci`：通过；包含 Go vet、Go 测试、UI typecheck、175 个 UI 测试和 UI build。
- 真实 split 进程：`just run` + `pnpm dev --host 127.0.0.1` 成功启动；`GET http://127.0.0.1:8787/healthz` 与 `GET http://127.0.0.1:3015/` 均返回 HTTP 200。
- 浏览器可视化 smoke：当前环境 browser runtime 无可用实例（`agent.browsers.list()` 返回空），因此未声称完成截图/交互验证；已记录为环境限制，不作为产品通过证据。

本次交付只产生审查文档/TODO 登记，没有修改 UI 或后端实现。
