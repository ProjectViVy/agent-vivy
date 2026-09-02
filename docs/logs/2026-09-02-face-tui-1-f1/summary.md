# FACE-TUI-1 F1 — 无网页控制面（gateway-less composition）

## What changed

VIVY-FACE-PACK（PR 2 / F1）验收句："启动器 / app 组合可在无 embed 时工作；审批在无 UI 时的失败路径有测试"。本片交付两条：

1. **组合拆耦合**（`internal/app/app.go`）：新增 `WithoutGateway() AppOption`。gateway 分支（浏览器 origin policy、`/rpc` mux、嵌入式 UI shell、`http.Server`）只在默认组合下构建；`WithoutGateway` 时进程完全没有 listener/embed，但控制面 handler 保留在 `App.control`。`Run`/shutdown 对 `httpServer == nil` 空安全。`vivy_headless` 组合（`internal/app/headless.go`）随之改为 `WithoutGateway()`——headless 此前会构建一个从不 serve 的 http.Server，现在按 §7 语义彻底不监听。
2. **进程内控制面**（`internal/app/facehost.go`）：`App.DialControl(ctx, notifications)` 用 `net.Pipe` + 既有 `controlrpc.NewJSONLTransport` + `NewPeer` 拉起 loopback JSON-RPC 对——宿主侧 handler 就是 web face 经 WebSocket 到达的同一个 `controlHandler`，零协议层改动（协议层本就 transport 无关）。

测试（`internal/app/facehost_test.go`，app 级组合测试，脚本化 Anthropic SSE 服务器 + frozen ENV 会话）：

- `TestLoopbackControlCompletesApprovedConversation`：无 embed/无 listener 的组合内，经 `DialControl` 走完 web face 同款方法链 `initialize → session/create → turn/start → approval/list → approval/respond(approved) → run/get(completed)`，journal 断言 `tool.finished` 零错误、终文落 `session/messages`。
- `TestGatewaylessRunWithoutFaceCancelsDurably`：审批挂起后无 face 应答，run 永不自愈；`run/cancel` 经同一进程内控制面把 run 收到 `cancelled`，gateway-less `App.Run` 在 ctx 取消后返回 nil。

## Key findings（钉进测试的事实）

- 只读工具在审批闸上被 `EvaluateApprovalPolicy` 恒自动放行（`internal/runtime/policy.go` "readonly or question tools do not require approval"）——governance prompt 规则对只读工具不构成挂起点。F1 审批轮次因此选 `write_note`（非只读，headless 审批测试的既有选型）。
- 手工构造的 `config.Config` 绕过默认值：`Runtime.WorkspaceRoot` 缺省为空会让引擎 agentsmd loader 拿到 nil filesystem backend 而崩；`Tools.Approval.Expiration` 缺省为 0 会让审批即时过期。测试配置里两者显式给出（真实装配路径由 config 默认值兜底）。
- 引擎流式调用模型：脚本服务器按 `"stream":true` 判别回 Anthropic SSE 转录，非流式请求仍回 JSON（对齐 `provider/claude_test.go` 的协议级测法）。
- 修一个真实回归隐患：`appOptions` 的 `gateway` 初值必须显式 `true`（零值 false 会让所有默认组合丢掉 HTTP server，`TestRPCBootstrapRoutePrecedesUIShell` 当场抓住）。

## Explicitly not done

- FaceHost / SDK Face 契约、出厂 `faces/tui` 器官、pack `face:` 配方键（FACE-TUI-1 后续片）。
- F2 headless 器官化（现 `vivy run` 走 `RunHeadless`，不依赖本片新增面）。
- §14 四问（faces/ 独立 go.mod、默认 face、网页与 TUI 同居、headless 审批失败退出）——按合同在 F2/F3 片前呈报拍板。
