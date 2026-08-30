# CH-C3 — acceptance（人怎么看出它成了）

日期：2026-08-30。

## 产品视角：第一只耳朵会动了——虽然还是假的

- 日常身体**仍然没有耳朵**：默认 `vivy.exe` `Register()=nil`、`channels:` 空缺 → 行为与 C2 前完全一致，零真实协议 HTTP。
- 变化在：把一只「假耳朵」放进这一代 + 写上配置，消息就能走完整条世界入口回路。

## 人可以亲手验证的点

1. **门是绿的**：`just ci` 退出码 0（含 channelhost TCK 8 项、runtime Provenance 回归）。
2. **回路成立（TCK 即演示）**：fake channel `Start` → `PublishInbound`（sender 在名单）→ Journal 出现 `channel.inbound` 事件（payload 五字段，无 token）→ 会话 `sess_ch_<hash>` 自动建立（标题 `channel/fake/chat-1`）→ 用户行 `Source=channel` 带三字段出处 → Run 被调起 → run 终态后假适配器 `Send` 收到最后一条 assistant 回复（恰好一次；重放终态不重发）。
3. **fail-closed 三连**：
   - `allow_from` 空 → 该通道**拒绝 Start**（错误日志，不入账、不建会话、不 Run）；
   - sender 不在名单 → 丢弃并记审计（不是错误），模型永远看不到；
   - `channels:` 写了身体里不存在的名字 → **整个启动失败**（对齐 `tools.enabled` 未知名字的既有语义）。
4. **本机 UI 与 channel 会话不合流**：channel 会话 ID 由 (channel, chat_id, topic) 哈希派生，UI 新建的 `sess_` 随机 ID 永远撞不上；同一 chat 重启后仍映射到同一会话（确定性派生，无映射表）。
5. **耳朵能被点名也能被拆下**：配置里不写 = 编入但不起；`enabled: false` = inspect 可见但不启动；从这一代删掉插件 = 启动失败提示（配置指向了不存在的名字）。

## 明确不属于本刀的验收（勿在此追讨）

- 真实 Telegram 收发 → CH-C4；钉钉 → CH-C6。
- inspect/设置页显示耳朵状态 → CH-C5。
- 崩溃后不丢回复（持久化出站队列）、`chanin_*` 事件保留策略、Secret 钉死 token_env → 已登记 §0.1，C4 起硬化。

## 回滚

去掉 app 装配（或 revert 本分支）即让耳朵消失；默认身体行为不变。
