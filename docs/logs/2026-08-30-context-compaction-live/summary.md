# 上下文压缩真实生效（2026-08-30）

## 变更内容

让上下文压缩从「演示/死配置」变成真实闭环：聊天框上下文环显示服务端真实占用，会话超支时由 Eino 原生官方中间件自动压缩，设置页「上下文压缩」卡片真实持久化并作用于引擎，手动压缩持久化为会话级摘要并折叠进后续 feed。

### 调研落点

采用 `docs/research/AGENT-LOOP-PORT-COMPARISON.md` §4.3-E2 / P3 结论：Eino v0.9.13 官方 `reduction` + `summarization` 中间件（完整版 + Vivy 桥接）；自研 `internal/runtime/compaction` 包保持原状（meter 口径被复用为 token 估算）。

### 内核（runtime / engine）

- 新 `internal/runtime/compaction_middleware.go`：
  - `ContextMonitor` 微中间件在 `BeforeModelRewriteState` 记录压缩前 token 数；
  - `buildCompactionHandlers` 装配 Eino 官方 `reduction`（`SkipTruncation=true`、`Backend=nil` 只做内存占位转存、`ClearRetentionSuffixLimit=keep_recent`）与 `summarization`（`Model=主模型`、`Trigger=trigger_tokens`、`EmitInternalEvents=true`、`Retry=0`、`Callback` 发事件）；顺序 = 先 reduction 后 summarization（测试钉死）。
- `engine.go`：`EngineConfig.Compaction *CompactionPolicy`；启用时把三个中间件追加到 handler 链。
- `mapper.go`：识别 summarization 中间件 `generate_summary` CustomizedAction，把携带 Usage 的事件映射为 `model.usage`（摘要调用可见入账）。
- `hooks.go` / `governanceSink`：`GovernanceEvent` 增压缩字段；`context.compacted` 分支持久化 + 发布；summarization 模式额外 `ReserveModelCall`（MaxModelCalls 不被绕过，P3②）。
- 新 `internal/runtime/compaction_policy.go`：`CompactionPolicy` + `TriggerTokens` = `min(max_tokens×threshold%, feed 字节预算 tokens)`，保证字节上限内可达触发。
- 新 `internal/runtime/compaction_service.go`：
  - `ContextStatus(sessionID)`：真实会话压力（feed bytes/tokens vs 模型窗口 vs 字节上限、would_compact、last_compaction）；
  - `CompactSession(sessionID)`：手动压缩——主模型生成摘要 → `session_compactions` 持久化 → `context.compacted`（mode=session）入 Journal；
  - `ScheduleEngineReload`：settings 保存后空闲立即重建引擎、在途延迟到下次空闲 run 起点；每次 run 顶层捕获 engine 引用。
- `runMessages` / `foldSessionHistory`：feed 装配时把已被会话摘要覆盖的旧行换成 `【会话压缩摘要】` 前缀的 user 消息，尾部保真。

### 契约

- `EventContextCompacted EventType = "context.compacted"`（非终端），词汇 35（`domain_test` 更新）。
- `payloadContextCompacted`（仅数字 + mode，D-010）。
- `schemas/events/run-event.schema.json` 枚举 + `payloads/context.compacted.json`。

### 配置与设置

- `config.example.yaml` `runtime.compaction`：`enabled`(默认 true) / `max_tokens`(0=模型窗口否则 128000) / `trigger_percent`(80) / `keep_recent`(12)；`config.go` 默认 + 校验（trigger 1..100、keep_recent≥1、max_tokens≥0）。
- `settings.yaml` `compaction` overlay（nil=用配置；空覆盖归 nil）；`Validate` 边界与 config 一致；`applySettingsOverlay` 启动时合并。
- `settings/get` 返回 `compaction`（有效值 + `config_*` 回退）；`settings/update` 读-改-写合并 compaction 段；保存后 `OnSettingsChanged` 比较有效策略，变化时 `ScheduleEngineReload`。

### 存储（迁移 015）

- `storage.CompactionStore`：`SaveSessionCompaction` / `LatestSessionCompaction`。
- sqlite `migration015` 建 `session_compactions`（PK session_id+created_at+run_id）；postgres schema v14 同表；两边 `compaction.go` 实现。

### RPC

- `session/context` → `ContextStatus`。
- `context/compact` → `CompactSession`（运行中 409；无可压缩/未超预算回 `skipped` 结果）。
- capabilities 与 `RPC_METHODS` 登记新方法。

### UI

- `api.ts`：`SessionContext` / `CompactResult` / `CompactionSettingsView` 类型 + `getSessionContext` / `compactSession`；`settingsUpdateFrom` 携带 compaction 段（防整文档替换清掉）。
- `store.ts`：`sessionContext` 状态 + `loadSessionContext`；selectSession / run 终态 / `context.compacted` 事件后刷新；`compactSession` action。
- `ChatView` / `ChatInput`：环改读真实 `session/context`（tokens vs 模型窗口 + 字节明细 + 「已达压缩阈值 / 已压缩」状态），删除客户端硬编码 256KB 估算。
- 新 `CompactionSettingsCard.tsx`（设置 → 通用）：启用开关、三个输入（config 回退占位）、真实占用条、「保存配置」「立即压缩」「刷新占用」；`DivaSettingsPreview` 里的演示「上下文压缩」卡退化为说明占位。
- i18n：`chatInput.context*` 改 tokens 文案；`settings.compaction.*` 改真实文案（zh/en）。

## 明确不做（本期）

- `reduction` 转存 Backend/offload 文件级恢复（`Backend=nil` 仅内存占位）→ `docs/TODO.md` CMP-1。
- 独立摘要模型 `summary_model` → CMP-2。
- 会话级摘要的检索 UI（摘要已进 feed，无检索面）→ CMP-3（G2 候选）。
- TurnLoop / P1 奖励轮 / 运行时 `/compact` 控制通道（第二批）；preflight 增补上下文字段（本次由 `session/context` 覆盖）。

## 变更文件

后端：`internal/domain/event.go`、`internal/runtime/{engine,service,mapper,hooks,payloads,preflight}.go`、新 `internal/runtime/compaction_{middleware,policy,service}_test*.go`、`internal/config/config.go`、`internal/app/settings/settings.go`、`internal/app/{app,compaction}.go`、`internal/storage/contracts.go`、`internal/storage/sqlite/{sqlite.go,compaction.go}`、`internal/storage/postgres/{schema.go,postgres.go,compaction.go}`、`internal/rpc/control.go`、`config.example.yaml`、`schemas/events/**` 及对应测试。

前端：`ui/src/components/settings/{CompactionSettingsCard.tsx（新）,SettingsView.tsx,DivaSettingsPreview.tsx}`、`ui/src/components/chat/{ChatView,ChatInput}.tsx`、`ui/src/lib/{api,store}.ts`、`ui/src/i18n/{zh,en}.ts`。