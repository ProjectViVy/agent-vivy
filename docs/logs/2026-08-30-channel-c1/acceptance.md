# CH-C1 — acceptance（人怎么看出它成了）

日期：2026-08-30。

## 产品视角：这一刀用户什么都没看见——这是对的

CH-C1 是阶段 A「遗传物质」：Journal 先认识世界入口，身体上还没有耳朵。所以：

- 本机 UI 对话（`http://127.0.0.1:3015` 或嵌入 `:8787`）**行为零变化**——发消息、收回复、审批照旧。用户行在账本里多了一个显式 `Source="ui"`，语义与之前（空=ui）完全一致。
- 没有 Telegram/钉钉/飞书/QQ/Discord 可配置、可连接——不存在的功能不出现在设置页。
- 默认 `vivy.exe` 身体未变：`Register()` 仍为 nil，`go.mod` 无任何平台 SDK。

## 人可以亲手验证的点（不看代码也能做）

1. **门是绿的**：在仓库根跑 `just ci`，退出码 0（Go 全包 ok + UI 21 文件/175 测试 + 构建成功）。
2. **户口本上有了新页**：打开 `schemas/events/payloads/channel.inbound.json`，能看到世界入口事件的定形字段（channel/chat_id/sender/message_id/session_id，run_id 可选），且没有任何 token/密钥/原始报文字段。
3. **旧账本不烂**：拿一个升级前的 SQLite Journal（`data/vivy.db` 所在引擎）启动新版，历史会话照常打开、消息照常显示——migration016 只是给 `messages` 表加了四个默认空串的列。
4. **出处语义**（给开发/验收者）：任何一条 Message，`Source` 为空或 `"ui"` 时按本机 UI 消息解读；带 `Channel/ChatID/ChannelMessageID` 的行将是（未来 C3 起）世界入口进来的消息。conformance `CN-17` 已把这两条规则钉进测试。

## 明确不属于本刀的验收（勿在此追讨）

- 收发真实平台消息 → CH-C4/C6/C7。
- inspect/设置页看到通道、开关身体里的耳朵 → CH-C5。
- `channel.inbound` 事件真正出现在 Journal → CH-C3（Host 才是发射方）。
- UI/JSON-RPC 上看到消息出处 → CH-C5（本刀 RPC 不投影出处）。

## 回滚

revert 本分支即可：无运行时耳朵，风险面仅限存储层四个新列（`DEFAULT ''`，向后兼容）与一个新事件枚举值（无发射方）。
