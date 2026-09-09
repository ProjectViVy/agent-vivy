# 2026-08-29 · Real PLAN / TODO display in the chat area

Date: 2026-08-29
Status: complete
Lane: `feat/plan-todo-display` worktree (`../agent-vivy-plan-todo-display`)

## Outcome

Following DSH's composer progress strip + collapsible list, the plan/todos on Vivy's chat page were changed from
`demo-api` fake data to real session-scoped todos from the kernel.

The authoritative data is the Journal `todos` table (`task_create` / `task_update` / `task_list`).
DSH `goal` / `todo_write` were not ported, and Diva `PlanRuntimeState` was not revived.

## Delivered

### RPC

- `session/todos`: lists this session's todos by `session_id` (excluding `metadata`).
- `ControlDeps.Todos`; production wiring `backend`.
- `Todos == nil` → method-not-found.
- capabilities adds `session.todos`.

### UI

- `api.listTodos` + store `todos` / `todosPhase` / `todoPanelOpen`.
- Selecting a session, a terminal run state, or a `task_*` `tool.finished` event refreshes the list.
- Above the chat input, `TodoProgressStrip` is hidden when empty; it displays the current in-progress title + status counts.
- Right-side `SessionTodoPanel`: current (pending / in_progress) and history (completed / cancelled).
- The desktop right rail collapses; narrow screens still use the top-bar Sheet. The same panel component is used.
- The top-bar todos button no longer calls `getPlanSidebarData`. Delete `PlanSidebarPanel`, which only served the fake path.

## Explicitly not done

- UI add/edit/delete of todos (to avoid fake operations).
- DSH `create_goal` / plan-mode review.
- Cross-session history.
- New journal event types.
- Merge back to `main` / push (requires user authorization).
