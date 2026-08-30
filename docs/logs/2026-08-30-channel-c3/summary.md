# CH-C3 — ChannelHost + 假插件 TCK（summary）

日期：2026-08-30。分支 `feat/channel-c3`（自 `feat/channel-c2` edf24c8 切出；顺序切片复用同一 worktree，分支逐刀独立）。
PLAN：`docs/plans/channel-epic/CH-C3.md`。合同：`VIVY-CHANNEL-PACK.md` §7/§8/§12。

## 做了什么

世界入口第一次真实存在：假 channel 发一条文本 → `channel.inbound` 入账 → `Message(source=channel)` → `Service.Run` → 终态投回适配器 `Send`。默认身体 `Register() = nil` 依旧无耳朵、零真实协议。

1. **新包 `internal/channelhost/`**（零 eino、零 internal/runtime import——importlint 自动覆盖 + 人工 grep 双确认）：
   - `host.go`：`StartAll`/`StopAll`/`Started`。fail-closed：未配置 = 编入不启动；`enabled: false` = 不启动；**空 `allow_from` 拒绝 Start 并记错误**；每只耳朵的 Start 失败只跳过该耳，不杀有机体。
   - `session.go`：会话映射（裁定：**确定性派生 ID** `sess_ch_<sha256(channel\0chat\0topic)[:16]>`，零新存储表、重启稳定、与 UI Session 天然不合流；标题 `channel/<名>/<chat>`）。并发首条消息的建会话竞态以重读收敛。
   - `dispatch.go`：PublishInbound 管线 = 信封解析（缺 channel/chat_id/sender/message_id 丢弃）→ allow_from 精确匹配（不在名单 = 丢弃记审计，不是错误）→ EnsureSession → **入账 `channel.inbound`**（裁定：每条消息独立伪 run 作用域 `chanin_<16hex>`，payload 逐字段对齐 C1 schema，`run_id` 省略；永不写终态）→ 文本 parts 提取 → `Run` 注入函数（带 `domain.Provenance`）→ 终态跟踪。终态投递：`run.completed` → 取该 run 最后一条非 tool assistant 行 → `Send`（脱离 runtime goroutine，`WithoutCancel` + 30s 超时）；failed/cancelled 本刀只记日志不投递。
   - `capabilities.go`：`Capabilities` 11 位 + `Discover`（对 sdk/plugin 能力接口逐一类型断言；TaskLifecycle/PipeServer 标记保留不断言）。
   - `channelenv.go`：Host 的 `plugin.ChannelEnv`——`Secret` fail-closed（缺 grant/空值报错，值永不入日志）、30s 出站 `http.Client`（无 Listen）、`PublishInbound` 入口、noop MediaStore。
   - `fake/`：测试 Channel（Start 即 Publish 一条 hello；Send 记内存；不实现任何可选能力）。
2. **TCK 8 项**（真实 sqlite 后端 + 注入 Run 桩 + Journal 录制装饰器）：空 allow_from 拒 Start 且后续入站丢弃；非名单 sender 不 Run 不建会话不入账；名单内 → `chanin_` 入账 + `Source=channel` 三字段全落 + Run 被调 + 会话 ID 确定性；终态投递恰好一次且重放终态不重发；failed 不投递；未知配置名启动失败；Discover 对零实现 fake 报空能力。
3. **runtime 出缝（service.go，未碰 engine.go）**：`RunOptions.Provenance *domain.Provenance`（新增 `domain.Provenance`）；nil 路径与 C1 的 `Source:"ui"` 字节等价，RPC/worker 等既有调用方零破坏（有测试钉住）。
4. **能力接口补全（sdk/plugin/channel.go，名字零改动）**：C2 的 9 个保留槽中的 7 个获得最小 v1 方法集（MediaStore/Typing/MessageEditor/Placeholder/MediaSender/WebhookHandler/StreamingCapable），新增 MessageDeleter/ReactionSender/ListenHandler/HealthChecker；TaskLifecycle/PipeServer 保持零方法（阶段 H）。每个注释「Minimal v1 surface; the first real adapter (C4) pins the ABI.」。
5. **app 装配**：`partitionChannels`——`channels.<name>` 对不上编入集合 = **启动失败**（对齐 tools.Resolve 先例）；channel seam 却不实现 Channel = 启动失败；Host 在 NewService 前构造并以结构化兼容挂进 `RunHook`；`StartAll` 在 `Recover` 后、listen 前（失败中止并关后端）；`StopAll` 在关停序列最前。Register()=nil 时行为与现在完全一致。

## 设计裁定记录（含 C1 遗留结案）

- **CH-C1-N1 结案**：`channel.inbound` 走伪 run 作用域入账，journal 双引擎零改动、D-008 不受影响，合同零改动；`run_events` 会积累无 run 行的 `chanin_*` 事件（惰性数据，Recover/ListActiveRuns 走 runs 表），保留策略登记 §0.1。
- **CH-C2-N2 部分结案**：4 个新能力接口落地；`InboundMessage`/`OutboundMessage` 的 run_id/task_id 槽**有意缓建**（伪 run 设计下本刀无消费者），SDK 注释已改写为 deferral 表述，TODO 行同步。
- 合同 §12 草图顺序（journal → Ensure）与 C1 schema（payload 必填 session_id）的张力 = 既有 CH-C1-N2；本刀实现为 Ensure 在前（schema 优先）。

## 明确没做（不做声明）

- 无真实协议、无 `vivy channel` 子进程（C8）、无 webhook/listen 服务面；Host 不认识 parse_mode/encrypt。
- 终态投递为内存跟踪：进程在 run.completed 与 Send 之间退出会丢回复；`chanin_*` 事件无清理；`Secret` 未钉死到信封 `token_env` 名单——全部登记 §0.1（CH-C3-N1/N2），C4 真适配器落地时硬化（Send/Stop 竞态安全也在 C4）。
- failed/cancelled 不向适配器投递任何文案（本刀产品决定，投递失败文案属 Face/后续）。
- 未 push。
