# Acceptance — FACE-TUI-1 F1

## 怎么看出来它成立（人验视角）

1. **进程内跑完一次带审批的对话，不开任何端口。**
   ```
   go test ./internal/app -run TestLoopbackControlCompletesApprovedConversation -v
   ```
   通过 = 在 `WithoutGateway()` 组合（无 embed、无 mux、无 origin policy、无 listener）里，一个普通 Go 客户端经 `App.DialControl` 用 web face 的同一批 RPC 方法创建了会话、发起回合、收到审批、批准、等 run 跑完，journal 里 `write_note` 零错误、终文在会话消息里。

2. **无 UI 时审批不会悄悄自愈，取消是持久出路。**
   ```
   go test ./internal/app -run TestGatewaylessRunWithoutFaceCancelsDurably -v
   ```
   通过 = 审批挂起后 run 一直停着；从同一进程内控制面发 `run/cancel`，run 状态落 `cancelled`，进程 `Run` 干净退出。

3. **headless 组合真的不再监听。**
   `TestLoopbackControl*` 内断言 `a.httpServer == nil`；`vivy_headless`（ci 的 headless-compile 步骤）现在经 `RunHeadless → WithoutGateway()` 组装，进程内不再有 `http.Server`。

4. **web face 一切照旧。**
   默认组合（不带新选项）与改动前逐字节等价——`just ci` 的 embedded-UI 路径、origin policy 测试（`TestRPCBootstrapRoutePrecedesUIShell`）全绿。

## 边界

- TUI 器官本身（faces/tui）、pack `face:` 键、FaceHost SDK 契约属后续片，不在本片验收范围内。
- 审批轮次使用 `write_note`：只读工具被审批闸恒自动放行是既有设计（`runtime/policy.go`），不是本片缺陷。
