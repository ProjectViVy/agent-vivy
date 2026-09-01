# CH-C1-N3 — Message 出处上 RPC/UI

## What changed

出处（Provenance）自 CH-C1 起随用户消息行入库（`domain.Message` 的
`Source`/`Channel`/`ChatID`/`ChannelMessageID`），但 `messageResult` 只投影
ID/RunID/Role/Content/CreatedAt，JSON-RPC 不可见。本切片补齐投影与显示：

- `internal/rpc/control.go`：`messageResult` 新增
  `Provenance *messageProvenanceResult`（`json:"provenance,omitempty"`）。
  `messageProvenance(message)` 按域规则投影：`EffectiveSource()=="channel"`
  才出对象（`{source:"channel", channel, chat_id, channel_message_id}`），
  ui 轮（含空 Source 的历史行/进程内追加）整体省略字段——与域层「nil
  Provenance 或空 Source 读作内置 UI」一致。`session/get` 与
  `session/messages` 两个投影点同接。
- `ui/src/lib/api.ts`：`MessageProvenance` 类型 + `Message.provenance?`。
- `ui/src/components/chat/MessageBubble.tsx`：user 消息带 provenance 时在
  气泡上方右侧渲染一行 10px muted 出处标记
  `<channel|source> · <chat_id>`（纯数据文本，channel 名/会话 id 是专有
  数据，不引入 i18n 键）；ui 轮零视觉变化。助手行不_stamp 出处，无标记。

## What was explicitly not done

- 出处词表校验（`Source` 透传任意非空值）＝ CH-C1-N4，待 CH-C2 SDK seam
  随合同定 `ui|channel` 词表，本切片不动。
- 合同 §12 payload 回写（digest/bytes vs identifiers-only）＝ CH-C1-N2，
  待架构师拍板；本切片投影的是 Message 行实有字段（无 sender——域里本就
  没有该字段）。
- UI 编辑/过滤入口：出处仅展示，不提供按渠道过滤。
