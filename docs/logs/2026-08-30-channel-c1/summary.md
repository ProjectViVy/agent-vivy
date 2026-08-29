# CH-C1 — 账本：`channel.inbound` + Message 出处（summary）

日期：2026-08-30。分支 `feat/channel-c1`（自 `feat/channel-super-contract` 82ecf14 切出，独立 worktree）。
PLAN：`docs/plans/channel-epic/CH-C1.md`。合同：`docs/architecture/VIVY-CHANNEL-PACK.md` §7.3/§8/§12。

## 做了什么

Journal 认识「世界从哪只耳朵进来说了什么」。身体上还没有耳朵：无适配器、无 ChannelHost、无 sdk/plugin ABI 变化，Eino 循环未动。

1. **Message 出处四字段**（`internal/domain/session.go`）：`Source`（`"ui" | "channel"`，空读作 ui）、`Channel`、`ChatID`、`ChannelMessageID`。新增 `EffectiveSource()`（空 → `"ui"`）与零值兼容单测（`TestMessageEffectiveSource`）。全仓 `domain.Message{` 字面量均为键名字面量，加字段零编译破坏（explore 扫描确认）。
2. **新事件类型**（`internal/domain/event.go`）：`EventChannelInbound = "channel.inbound"`，登记进 `EventTypes`（35→36）；非终态，`Terminal()`/`RunStatus()` 未动。`TestEventVocabulary` 同步 36。
3. **事件 schema**：新建 `schemas/events/payloads/channel.inbound.json`（`channel/chat_id/sender/message_id/session_id` 必填，`run_id` 可选，`additionalProperties: false`；无 token、无原始 webhook body、无内容字段）。`run-event.schema.json` 枚举加 `"channel.inbound"`，与 `EventTypes` 逐项镜像。
4. **存储迁移（双引擎）**：
   - sqlite：`migration016` 四条 `ALTER TABLE messages ADD COLUMN ... TEXT NOT NULL DEFAULT ''`（沿用 migration011 模式；测试经 `Open` 走全迁移链）。
   - postgres：`schemaVersion` 14→15；`migrate()` 增加原地升级分支——已记录 version 14 的库执行 `schemaV15Upgrade`（同四条 ALTER），全新库走全量 `schemaV15` DDL。存量 postgres 库因此不被甩下。
   - 两处 `messages.go`（INSERT/SELECT/Scan，9→13 列）保持字节级同构。
5. **conformance CN-17**「message provenance round-trip」：channel 行四字段精确往返 + 空 Source 行读回 `EffectiveSource()=="ui"`；guard 16→17。双后端自动挂载。
6. **UI 路径**：`internal/runtime/service.go` `RunWithOptions` 用户行显式 `Source: "ui"`；assistant/tool 投影保持空（空=ui），语义不变。
7. **升级路径测试**：`internal/storage/postgres/upgrade_test.go` 用 82ecf14 冻结的 v14 DDL 手工搭库 → 经生产 `OpenSchema` 原地升级 → 断言版本记录 `[14 15]`、四列 `NOT NULL DEFAULT ''`、旧行存活且 `EffectiveSource()=="ui"`、带出处写入可往返。
8. **文档一致性**：`docs/AGENT-VIVY-ARCHITECTURE-V0.md` conformance 计数 CN-01..CN-16 → CN-01..CN-17。

## 与合同的差异（照 PLAN 执行，待架构师追认）

合同 §12 的 Journal 草图写 `{channel, peer, message_id, content_digest, bytes}`；CH-C1 PLAN §4 定形为 `{channel, chat_id, sender, message_id, session_id, run_id?}`（peer 拆为 chat_id+sender，无 content_digest/bytes）。按权威顺序（PLAN 为本切片开工令）执行，已在 `docs/TODO.md` §0.1 登记请架构师确认是否回写合同。

## 明确没做（不做声明）

- 无 ChannelHost（禁止建 `internal/channelhost`）、无五个适配器、无平台 SDK、`go.mod` 零新增依赖。
- `sdk/plugin`、`internal/pluginhost`、`plugins/`、`internal/generated/plugins/zz_register.go`（仍 `return nil`）零改动。
- RPC `messageResult` 不投影出处（UI/JSON-RPC 不可见）——留给 CH-C5 inspect/设置页切片，已登记 §0.1。
- `Source` 无类型词表校验（透传任意非空值）——C2 SDK seam 落地时随合同定词表，已登记 §0.1。
- 未加 `channel.inbound` 的 Go payload 结构体与发射器（无 Host 即无发射方；C3 随 mapper 一起长）。
- `run-event` envelope 必填 `run_id` 与 `channel.inbound`（发生在 run 存在前）的信封张力——C3 设计前拍板，已登记 §0.1。
- 未 push。
