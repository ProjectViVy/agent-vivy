# 2026-08-29 · 聊天区真实 PLAN / TODO 显示

Date: 2026-08-29
Status: complete
Lane: `feat/plan-todo-display` worktree (`../agent-vivy-plan-todo-display`)

## Outcome

对照 DSH 的 composer 进度条 + 可折叠清单，把 Vivy 聊天页的计划/待办从
`demo-api` 假数据换成内核真实的 session-scoped todos。

权威数据是 Journal `todos` 表（`task_create` / `task_update` / `task_list`）。
没有移植 DSH `goal` / `todo_write`，也没有复活 Diva `PlanRuntimeState`。

## Delivered

### RPC

- `session/todos`：按 `session_id` 列出该会话 todos（不含 `metadata`）。
- `ControlDeps.Todos`；生产接线 `backend`。
- `Todos == nil` → method-not-found。
- capabilities 增加 `session.todos`。

### UI

- `api.listTodos` + store `todos` / `todosPhase` / `todoPanelOpen`。
- 选会话、run 终态、`task_*` 的 `tool.finished` 刷新列表。
- 聊天输入框上方 `TodoProgressStrip`：空则隐藏；显示当前进行中标题 + 状态计数。
- 右侧 `SessionTodoPanel`：当前（pending / in_progress）与历史（completed / cancelled）。
- 桌面右轨折叠；窄屏仍用顶栏 Sheet。同一面板组件。
- 顶栏待办按钮不再调用 `getPlanSidebarData`。删除仅服务假通路的 `PlanSidebarPanel`。

## Explicitly not done

- UI 增删改待办（避免伪操作）。
- DSH `create_goal` / plan-mode review。
- 跨会话历史。
- 新 journal 事件类型。
- 合并回 `main` / push（需用户授权）。
