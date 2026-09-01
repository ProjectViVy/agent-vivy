# Acceptance — CH-C1-N3 provenance

## 人工如何确认

1. 绑定一个 channel（如 telegram 插件）后，从该平台给 Vivy 发一条消息。
2. 在 web UI（`http://127.0.0.1:3015`）打开同一会话：该用户消息气泡上方
   应出现一行小字出处标记（如 `telegram · chat-1`）。
3. 在 UI 里直接发送的消息：无任何出处标记（与改动前渲染逐像素一致）。
4. API 面：`session/messages` / `session/get` 返回里，channel 用户消息带
   `"provenance":{"source":"channel","channel":"…","chat_id":"…",
   "channel_message_id":"…"}`；ui 消息无 `provenance` 键。
5. 历史数据（改动前入库的空 Source 行）：读作 ui，无标记，不报错。

## 回归面

- 既有消息列表/附件渲染（data URL 图像）不变——ui-e2e runtime.spec 全绿。
- RPC 契约为纯增量字段（omitempty），旧客户端不受影响。
