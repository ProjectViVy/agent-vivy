# CH-C1 — 账本：`channel.inbound` + Message 出处

## 1. 身份

| | |
|---|---|
| ID | CH-C1 |
| 阶段 | A 遗传物质 |
| 人日 | 2 |
| 里程碑 | M-CH1 地基 |
| 依赖 | CH-0（已 DONE） |
| 后继 | CH-C2 |
| 分支 | **新** `feat/channel-c1`（不要写进 `feat/channel-super-contract`） |
| 合同 | `VIVY-CHANNEL-PACK.md` §7.3、§8、C1 |
| 演进 | `VIVY-CHANNEL-EVOLUTION.md` 阶段 A |

开工前：`00-standing-orders.md`。

## 2. 目标

Journal 能记下「世界从哪只耳朵进来说了什么」。本机 UI 对话不变。身体上还没有耳朵。

成功：`just ci` 绿；新事件 schema 存在；旧 Message 读出来 `Source=ui`；无适配器、无 Host、无 sdk/plugin 行为变化。

## 3. 现状

- `internal/domain/session.go` `Message`：ID/SessionID/RunID/Role/CreatedAt/Content/Tool*。无出处。
- `internal/domain/event.go` 无 `channel.*`。
- `schemas/events/payloads/` 无 `channel.inbound.json`。
- `internal/storage/sqlite/messages.go` INSERT 九列；`postgres/schema.go` `CREATE TABLE messages` 同样。
- `internal/runtime/service.go` `RunWithOptions` 直接 `AppendMessage` 用户行，无 Source。
- 无 ChannelHost。

## 4. 目标结构

本切片只长 L0 遗传物质，不长 Host。

```text
domain.Message
  + Source            "ui" | "channel"（空读作 ui）
  + Channel           平台名；ui 行空
  + ChatID            会话键一部分；ui 行空
  + ChannelMessageID  平台 message_id；ui 行空
  禁止：token、raw JSON blob、平台私有 metadata 当主列

domain.EventChannelInbound = "channel.inbound"
  payload: channel, chat_id, sender, message_id, session_id, run_id?
  不含密钥、不含原始 webhook body

messages 表：新列 DEFAULT ''，旧行 = ui
```

## 5. 文件清单

**改**

- `internal/domain/session.go` — Message 字段 + 校验
- `internal/domain/event.go` — EventType
- `schemas/events/payloads/channel.inbound.json`
- `schemas/events/run-event.schema.json` — enum 加类型（若有）
- `internal/storage/sqlite/sqlite.go` — messages DDL / 迁移
- `internal/storage/sqlite/messages.go` — INSERT/SELECT
- `internal/storage/postgres/schema.go` + `postgres/messages.go`
- `internal/storage/conformance` — 出处往返
- `internal/runtime/service.go` — UI 路径显式 `Source: "ui"`（或空=ui）
- 触及 Message 字面量的测试夹具

**禁止碰**

- `sdk/plugin`、`internal/pluginhost`、`plugins/`、平台 SDK
- `internal/runtime/engine.go` 的 Eino 循环
- UI 设置页

## 6. 步骤

1. 给 `Message` 加字段；空 Source 视为 `ui`。单测：零值兼容。
2. 加 `EventChannelInbound` + JSON schema。payload 不得含 token。
3. sqlite/pg：`ALTER` 或重建测试库 DDL，DEFAULT `''`。conformance：写入 channel 行再读回。
4. `Service.Run` UI 路径：Source 保持 ui。现有 runtime 测试必须绿。
5. 不实现 Host。可加纯函数测试「构造 inbound payload」。
6. `just ci`。
7. `docs/logs/YYYY-MM-DD-channel-c1/`。TODO CH-C1 → DONE。

## 7. 验收

- `just ci` 绿。
- 无 `telego` 等出现在物种 `go.mod`。
- 新库 Message 无出处列读失败 = 不合格；旧行必须还能 List。
- 事件 schema 有 `channel.inbound`；夹具不含密钥。
- UI 对话（runtime 测试）不要求新字段也能跑。

## 8. 禁止

- 建 `internal/channelhost`。
- 改 `sdk/plugin` ABI。
- 把 `Metadata map[string]string` 当主合同。
- 密钥进 Journal / 事件。
- 把本切片和 C2 混在一个 commit。

## 9. 风险与回滚

- sqlite 测试库是全量 DDL 而非迁移：两处 schema 都要改。
- Message 结构体字段加多会破未具名复合字面量：全仓搜 `domain.Message{`。
- 回滚：revert 本分支；无运行时耳朵，风险限于存储列。

## 10. 交接

完成后：下一 AGENT 读 [CH-C2.md](CH-C2.md)。C2 依赖本切片的 Message 出处字段名，不要在 C2 改名。
