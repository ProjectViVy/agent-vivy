# UI-TRAJ / UI-TRAJECTORY-DEMO — 会话真实轨迹 RPC + 面板接线

## Summary

把「中控台 → 轨迹」面板从纯演示数据切换为内核真实数据，交付内核
`trajectory/session` RPC 能力（UI-TRAJ）并完成面板换真实 api（UI-TRAJECTORY-DEMO）。

### Kernel（internal/）

- `internal/runtime/trajectory.go`（新）：`Service.SessionTrajectory` 把一个会话的
  最近 N 个 run（默认 20、上限 50）的 journal（run_events）+ 消息存储投影为
  turn 级轨迹结构。语义：
  - 一个 run == 一个用户回合（turn）；回合内每个模型调用是一个 Step；
  - 折叠事件：`run.started`（首个 run 产出 Session 区段 system 行，后续 run 仅更新
    provider/model 出处）、`model.request`（开请求槽）、`model.usage`（token）、
    `provider.retry`（重试计数）、`model.completed`（关请求槽 + ASSISTANT 行）、
    `tool.requested/started/finished`（工具行，入参/结果入详情）、
    `context.compacted`（turn 为 null 的 Compaction 行）、`run.failed`（Run 组失败行）；
  - 用户文案来自 Messages 存储按 RunID 关联（非 journal 重放）；
  - 有界投影：文本/详情截断 8 KiB（UTF-8 安全），悬空请求收口为 error。
  - 请求载荷只含哈希与字节长度（D-010），不做超出持久化内容的转录重组。
- `internal/rpc/control.go`：新增 `trajectory/session` 路由与 handler
  （session_id 必填、limit 可选；Service 未配置 → MethodNotFound；会话无 run → 404 语义保留给 ErrNotFound，空投影正常返回）。
- 测试：`internal/runtime/trajectory_test.go`（手工两 run 投影结构断言、limit 钳制、
  真实 Echo run 端到端投影）、`internal/rpc/control_test.go` 的
  `TestTrajectorySessionRoute`（缺参 InvalidParams + 播种 run 的形状断言）。

### UI（ui/src/）

- `ui/src/lib/api.ts`：`trajectory/session` 进 RPC_METHODS；snake_case wire 类型
  （TrajectorySessionWire / TrajectoryRecordWire / TrajectoryRequestWire /
  TrajectoryTokensWire）与 `fetchSessionTrajectory`。
- `ui/src/components/trajectory/trajectory-types.ts`（新）：面板展示层类型的唯一来源
  （TrajectoryCellKind、TRAJECTORY_KIND_LABEL、TrajectoryRecord、TrajectoryRequest 等），
  请求类型补充真实 RPC 携带的 `messages` / `preambleBytes`。
- `ui/src/components/trajectory/trajectory-session.ts`（新）：wire → 展示层映射
  （snake_case → camelCase、usage 缺省补 0、Compaction 组请求标记 purpose）。
- `ui/src/components/trajectory/TrajectoryPanel.tsx`：会话选择器（listSessions +
  Select，默认第一个会话）+ 刷新按钮；加载骨架 / 无会话空态 / 错误态
  （DemoLoadError）+ 折叠、搜索、选区、详情等既有交互全部作用于真实数据；
  演示辅助函数（requestByNumber / recordForRequest）由真实数据上的本地查找替代。
- 其余轨迹子组件（Toolbar/Timeline/Ledger/DetailPanel）仅改类型导入来源。
- `ui/src/components/trajectory/trajectory-demo-data.ts`：降级为
  `trajectory-utils.test.ts` 专用测试夹具（生产链路不再引用，bundle 不含）。
- i18n：`dashboard.trajectoryDesc` 改为真实数据描述；trajectory 段新增
  `pickSession` / `sessionEmpty` / `refresh`（en + zh 同步）。

## Explicitly not done

- 详情面板未展示 `messages` / `preambleBytes`（类型已就位，UI 呈现留待需要时再加）。
- 轨迹投影不包含 token 成本（cost）——journal 中无价格事实，不做推算。
- DSF `context` / `subtool` kind 仅为渲染兼容保留，Vivy 内核不产出这两类记录。
- run 内的中间 thinking 内容不在投影里（journal 不持久化明文思考流）。
