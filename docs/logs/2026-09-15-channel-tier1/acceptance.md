# Acceptance — channel tier-1

How a human can tell each feature works. Browser checks run against the
split dev pair (`just dev`, open http://127.0.0.1:3015, Settings → Channels);
no production journal is touched.

## Feature 1 — failed deliveries are visible and revivable

1. Point a channel at a dead transport (e.g. revoke a token or block the
   network), let a chat message open a run, and let the reply delivery burn
   its attempts.
2. Open Settings → Channels: a "失败投递" (Failed deliveries) section lists
   the run id, channel, chat id, and the attempt count.
3. Restore the transport, press 重投 (Redeliver) on the row: the reply
   arrives in the chat within seconds and the row disappears.
4. While the channel is still dead (or the process is shutting down), the
   button reports the server's refusal instead of pretending to succeed.

## Feature 2 — ears show live health

1. Settings → Channels: a started channel with a live transport shows a
   green "连接正常 / Healthy" badge on its card and in the editor pane.
2. Cut the transport of a supervised channel (dingtalk/qq/feishu/discord):
   within a redial cycle the badge turns amber "重连中 / Reconnecting"
   (rate-limited platforms would show 平台限流); when the transport returns,
   the badge returns to green on refresh.
3. `curl` the control RPC `channel/inspect`: each started entry carries
   `health: {ok, class, detail}`; unstarted entries carry `health: null`.

## Feature 3 — approvals reach the chat, decisions stay kernel-owned

1. In a chat that is allow-listed on a channel, send a message that triggers
   a tool needing approval (e.g. a write outside the auto-approved policy).
2. The chat receives one text: "⏸ 等待人工审批:<tool>(会话 <session>)。
   回复 /approve <id> 批准、/deny <id> 拒绝;也可在本地客户端处理。"
3. Send `/pending`: the chat lists this session's pending approvals with
   short ids and expiry; approvals from other chats do not appear.
4. Send `/approve <id>`: the chat answers "✅ 已批准 <tool>(<id>)." and the
   suspended run resumes to completion; the local UI's Review queue shows
   the same decision, attributed to `channel:<channel>:<sender>`.
5. Send `/deny` instead: "❌ 已拒绝 …", the run's tool call is denied.
6. A second chat (its own session) cannot see or decide the first chat's
   approval even with the full id — the command answers "没有待审批".
7. Any other "/" text (e.g. "/tmp shows a path") opens an ordinary run —
   commands never swallow normal conversation.
8. Red lines hold: there is no approval surface in the channel beyond these
   text commands; every decision appears in the journal with its actor, and
   the local UI can decide first (the channel then answers 审批失败 with the
   already-decided error).
