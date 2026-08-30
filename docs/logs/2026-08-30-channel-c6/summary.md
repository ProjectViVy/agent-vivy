# CH-C6 — `plugins/dingtalk` Stream 单聊文本（summary）

日期：2026-08-30。分支 `feat/channel-c6`（自 `feat/channel-c5` d3d4342 切出；顺序切片复用同一 worktree）。
PLAN：`docs/plans/channel-epic/CH-C6.md`。合同：`VIVY-CHANNEL-PACK.md` §9/§11/§14.1（dingtalk 行）。对照：`.workspace/picoclaw/pkg/channels/dingtalk`（改写，零 import）。

## 做了什么

第二只真耳朵：钉钉 Stream（出站 WS 客户端）单聊文本闭环，目录形状照抄 telegram 样板。默认 EXE 零钉钉依赖。

1. **独立 module** `plugins/dingtalk`（`dingtalk-stream-sdk-go v0.9.1`；直接依赖仅 agent-vivy + SDK，gorilla/websocket 保持 indirect）。
2. **Stream 生命周期**：SDK 自动重连关闭（其后台重连循环会活过 Stop），改由插件侧 3s 上下文感知重拨监督器接管；Stop 幂等 + 有界等待 + 停止后重检关掉竞态 socket；**迟到回调栅栏**（SDK 用 `context.Background()` 派发帧，Stop 后到达的帧直接丢弃，不 rememberWebhook / 不 PublishInbound——评审 should-fix 已修 + 测试钉住）。
3. **入站**：仅单聊（`conversationType "1"`）纯文本；`content.content` 兜底、bot 自环防护（`ChatbotUserId`，比 picoclaw 多一层）、sender 兜底序与 picoclaw 一致；`Sender = "dingtalk:<staffId>"`。sessionWebhook 只存插件侧内存（按会话最新覆盖），永不进内核配置/信封/RPC；错误链中 webhook URL 脱敏（query 含 access_token，剥除——含 NewRequest 解析失败路径，D-010 双路封死）。
4. **出站**：`Send` POST 存下的 sessionWebhook（`{"msgtype":"text","text":{"content":...}}`，对照 SDK `SimpleReplyText` 修正了初稿的字形）；errcode/errmsg 处理；未知会话（重启后无 webhook）fail-closed 报错。无 markdown/卡片/群。
5. **凭据扩展（通用信封修复）**：`hostEnv.Secret` 现接受信封 `token_env` **或** settings 顶层 `*_env` 声明名（站立命令的 `token_env / *_env` 模式）；dingtalk 以 `client_id_env` + `client_secret_env` 双密钥声明；嵌套/非字符串声明忽略（fail-closed），值永不进日志。telegram 既有语义不变（其 settings token_env == 信封名）。
6. **CH-C2-N1 清账**：补齐四个 verify 夹具（transport=webhook / 重复 grant / tool seam 领 channel 族 grant / 负 max_message_runes），断言各对应规则，`go test ./sdk/...` 绿。
7. **真实 SDK 回环测试**：手写 RFC 6455 网关（stdlib，零新依赖）跑真 SDK 客户端全链：ticket 换取 → 握手 → CALLBACK 帧 → ack → PublishInbound → Send → Stop 后不重拨。`-race` 干净。

## 明确没做（不做声明）

- 群触发 / @列表 / markdown 回复 / 卡片 / 媒体（§14.3 第一刀明确排除）。
- HTTP webhook 机器人形态（Octos 基线拒绝）；`channel.webhook` grant 未启用。
- 死耳静默重拨（SDK 默认 logger 不输出，插件无日志面）——登记 §0.1 CH-C6-N1。
- settings `*_env` 名无 `^[A-Z_][A-Z0-9_]*$` 强校验（现为字符串即声明；适配器自身 strict decode 约束了实际使用）——登记 §0.1 CH-C6-N2 候选硬化。
- Stop 与在途重拨互斥的最长 ~50s 窗口（SDK mutex + gorilla 拨号 ctx 盲）——已文档化，无泄漏（-race 验证）。
- 真钉钉组织手工冒烟未做（无凭据，不挡 ci；回滚 = 配方不点名）。
