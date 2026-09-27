# Channel-native approval UI survey

Date: 2026-09-15 · Scope: whether QQ / Feishu / DingTalk can carry a native
approval card for the HITL channel surface, and what that would cost.
Conclusion: this generation ships text commands only; native cards are a
separate future contract.

## Findings per platform

### QQ (official bot, current plugin = C2C passive reply)

- Markdown in single chat and group chat is now open to all bots without a
  separate template application; the guild/channel scenario still requires
  internal access.
- Buttons / keyboard templates still require a separate permission
  application ("消息按钮模板"), so a native Approve/Deny card cannot be
  relied on for a self-serve deployment.
- Passive replies have a hard validity window: group chat ~5 minutes, C2C
  single chat ~60 minutes, with a reply-count cap per message. A run can be
  suspended far longer than 5 minutes, and group-window replies can expire
  before a human even looks — text commands sent as a NEW passive reply
  within the 60-minute C2C window are the only broadly available path.
- Error surface: `40034127` (no markdown template permission),
  `40034128` (passive reply window/count exceeded), `40054002` (bot muted).

### Feishu / Lark (current plugin = lark websocket)

- Interactive cards with button callbacks are the best native-UI candidate:
  card action callbacks can be received over the same WebSocket long
  connection the ear already uses (no public endpoint, no webhook).
- The card is a structured message — outside this generation's text-only
  outbound contract — and needs card-instance create/update plus a callback
  router; a meaningful slice of its own.

### DingTalk (current plugin = Stream robot)

- The modern path is the new interactive card system with callback type
  `STREAM` — card callbacks arrive over the existing Stream connection
  ("零公网 IP"); the old ActionCard belongs to the legacy webhook/callback
  mechanism the channel batch deliberately avoided.
- Same shape as Feishu: structured card + callback routing = a dedicated
  slice, not a by-product.

## Decision

Text slash commands (`/approve` / `/deny` / `/pending`) are the only
universally available surface across all five platforms this generation —
QQ's template permission requirement and short passive-reply window make
buttons non-dependable, while Feishu/DingTalk card callbacks, though
technically clean over the existing sockets, are structured-message work
outside the text-only outbound contract. Native card UIs stay a future
topic tracked in `docs/TODO.md` §0.1.

## Sources

- QQ open platform — send group message (markdown/buttons, passive window,
  error codes): https://bot.q.qq.com/wiki/develop/api-v2/autogen/api/v2_groups_group_openid_messages.post.html
- QQ open platform — send C2C message (60-minute passive window):
  https://bot.q.qq.com/wiki/develop/api-v2/autogen/api/v2_users_user_openid_messages.post.html
- QQ open platform — markdown message availability:
  https://bot.q.qq.com/wiki/develop/api-v2/server-inter/message/type/markdown.html
- Koishi forum — community notes on QQ button template permission:
  https://forum.koishi.xyz/t/topic/6737
- Feishu open platform — card callback communication:
  https://open.feishu.cn/document/feishu-cards/card-callback-communication
- Feishu open platform — handle callbacks over WebSocket:
  https://open.feishu.cn/document/server-side-sdk/python--sdk/handle-callbacks
- DingTalk open platform — respond to card callbacks in Stream mode:
  https://open.dingtalk.com/document/development/intelligent-assistant-with-interactive-card-use-tutorial
- DingTalk developer docs — Stream mode introduction (bot/event/card
  callbacks): https://open.dingtalk.com/document/resourcedownload/introduction-to-stream-mode
