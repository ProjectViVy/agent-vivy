# 验收（人的视角）

## 怎么确认装上了

1. `vivy-sdk pack --with feishu --out dist/` 后
   `vivy-sdk inspect-artifact dist/<gen>/`，`recipes.plugins` 里出现
   `"feishu"`——这一代身体里有飞书耳朵。
2. `channels.feishu.settings` 未知键会被拒绝（fail-closed）：塞一个
   `{"wat":1}` 进 settings，Start 拒绝启动。

## 怎么确认听得见 / 说得出

1. 在飞书开放平台建自建应用（机器人能力），配好
   `FEISHU_APP_ID` / `FEISHU_APP_SECRET`，配置
   `allow_from: ["feishu:<你的 open_id>"]`（open_id 可从首次事件日志
   或通讯录 API 取）。
2. 用个人号给机器人发一条私聊文本 → Vivy 日志出现该入站事件的
   journal 记录（channel=feishu，sender=feishu:ou_...）→ 运行完成后
   机器人在同一个 p2p 会话里回一条纯文本（`im.v1.messages`，
   `receive_id_type=chat_id`）。
3. 群里 @ 机器人发消息 → **无反应**（群聊不在第一刀范围）。
4. 把 `FEISHU_APP_SECRET` 改错再启动 → Start 直接失败闭合（首连
   被网关拒绝），不会留下一个"聋耳朵"假装在线。
5. 断网几分钟再恢复 → 监督循环以新 client 重拨，收发自愈；Stop /
   重启后没有对旧连接的复活或泄漏（`-race` 全绿）。

## 国际版

`settings.is_lark: true` + 国际版 Lark 应用凭据 → 同样流程走
`open.larksuite.com` 域名。

## 已知边界（按设计）

- 仅单聊纯文本；群聊、卡片、图片、表情回复不进不出。
- lark SDK 依赖树不支持 386 目标（编译失败是硬约束）。
