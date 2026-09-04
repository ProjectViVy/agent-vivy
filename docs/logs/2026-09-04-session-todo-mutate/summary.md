# 2026-09-04 · 会话待办状态修改 (Session Todo Mutation)

Date: 2026-09-04
Status: complete
Lane: `feat/ui-todo-mutate` worktree (`agent-vivy-ui-todo-mutate`)

## Outcome

闭环了此前 `docs/logs/2026-08-29-chat-plan-todo-display` 中明确标注为暂不实施的 UI 修改待办能力。
通过权威的控制面 RPC `session/todo/update` 与 UI 侧的乐观更新 + 失败回滚机制，允许用户在会话空闲期直接干预、更新 session-scoped todos 的状态（完成、取消、重新恢复）。

## Delivered

### 1. 后端控制面 RPC (`internal/rpc/control.go`)
- 注册并暴露 `session/todo/update`，并在 `capabilities` 声明中追加 `session.todo.update`。
- 请求参数接收 `session_id`, `id` (兼容 `todo_id`), `status`。
- **状态校验**：支持 `pending`, `in_progress`, `completed`, `cancelled` 四种有效状态。
- **运行期守卫**：
  - 会话存在性检查（`CodeNotFound`）。
  - 检查活跃 runs（`deps.Runs.ListActiveRuns`），会话存在未完成 run 时返回 `CodeConflict` ("session has an active run")，杜绝与模型工具执行发生并发写入冲突。
  - 单一进行中互斥约束：若尝试将某个任务标记为 `in_progress`，检查当前会话是否存在其他 `in_progress` 任务，若有则返回 `InvalidParams` ("only one task may be in_progress")。
- 更新成功后持久化并返回更新后的待办 DTO。
- 测试覆盖：在 `internal/rpc/control_test.go` 中新增 `TestControlHandlerUpdatesTodo`。

### 2. 前端客户端与状态流 (`ui/src/lib/`)
- `ui/src/lib/api.ts`：在 `RPC_METHODS` 中登记 `session/todo/update`，导出 `updateTodo(sessionId, id, status)`。
- `ui/src/lib/store.ts`：新增 `updateTodoStatus(todoId, status)`：
  - 拦截保护：若当前有活跃 run (`runActive(currentRun)`)，直接阻断不发送请求。
  - 乐观更新：本地状态预先更新并清空错误。
  - 异常回滚：若 RPC 调用失败，自动恢复先前的待办状态并将错误信息写入 `todosError`。
- 单测：在 `ui/src/lib/api.test.ts` 与 `ui/src/lib/store.test.ts` 中补充更新、运行期拦截及错误回滚用例。

### 3. UI 交互与多语言 (`ui/src/components/planning/SessionTodoPanel.tsx`)
- 待办列表项（`TodoRow`）：
  - 左侧状态替换为可操作的 `Checkbox`：已完成状态打勾，点击可切换 `completed` 与 `pending`。
  - `in_progress` 任务在非禁用态保留微调能力，在执行中显示加载动画。
  - 右侧悬浮操作按钮：对未取消条目显示 `Ban` 图标（标记为 `cancelled`），对已取消条目显示 `RotateCcw` 图标（恢复为 `pending`）。
  - 状态同步：完成条目弱化字体，取消条目添加删除线。
- 运行中与繁忙期锁定：
  - 面板顶部展示“执行中锁定”（`todos.runningLocked`）徽章。
  - 操作控件在 `isRunning || runBusy` 时自动设置 `disabled`，防止用户误触。
- 多语言 (`ui/src/i18n/{zh.ts,en.ts}`)：补充切换完成、取消、恢复待办及运行期锁定的中英文提示文案。
- 自动化测试：新增 `ui/src/components/planning/SessionTodoPanel.test.tsx`（6 个测试用例）。

### 4. 测试与平台稳定性
- `internal/codeface/launch_test.go`：在 Windows 平台环境下补齐 `filepath.EvalSymlinks` 处理，修复绝对路径匹配断言。

## Explicitly not done

- 随意任意添加或删除待办项（待办的权威结构与步骤规划依然由模型任务规划生成，本次仅开放人类干预状态闭环）。
- 跨会话待办修改。
- 绕过 Run 状态机的强制并发修改。
