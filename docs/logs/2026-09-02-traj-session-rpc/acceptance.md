# Acceptance — UI-TRAJ / UI-TRAJECTORY-DEMO

## 人工验收路径

1. `just run`（控制面 8787）+ `cd ui; pnpm dev`（Vite 3015），打开
   `http://127.0.0.1:3015`。
2. 进入「中控台 → 轨迹」：
   - 顶部出现「会话」选择器；已有会话时默认选中第一个；
   - 加载期间显示骨架；失败显示可重试错误条。
3. 选中一个有过对话的会话：
   - 时间轴出现三泳道条带，账本出现 Session/User/ASSISTANT/TOOL 行；
   - 点击 ASSISTANT 行右侧弹出详情（Summary/Usage/Timing），数字与该会话
     Token 统计口径一致（同一 journal 事实源）；
   - 工具行点击可见入参/结果详情；错误工具行呈红色。
4. 切换另一个会话：账本内容随会话切换（不复用上一会话的折叠/搜索状态）。
5. 全新空库（无会话）：显示「暂无会话；发起一轮对话后即可查看轨迹。」
6. 有会话但该会话无 run（例如仅创建未对话）：账本显示「暂无轨迹记录」空态。

## Kernel 侧可观察行为（无 UI 依赖）

- `trajectory/session` RPC：`{"session_id": "<id>"}` 返回
  `{session_id, turns, records, requests}`；缺 session_id → InvalidParams；
- 记录 kind 为闭合集合（system/user/message/tool/compacted），
  文本字段超 8 KiB 截断并带 `[truncated]` 尾标。

## 边界

- Token 统计与轨迹投影同源（run_events），两者数字应一致；若不一致，
  以 journal 为准并在轨迹侧报 bug。
