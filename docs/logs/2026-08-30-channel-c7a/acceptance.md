# Acceptance (human view)

## Confirming it is packaged

1. After `vivy-sdk pack --with feishu --out dist/`, run
   `vivy-sdk inspect-artifact dist/<gen>/`; `recipes.plugins` contains
   `"feishu"`—this generation's body has a Feishu ear.
2. Unknown keys in `channels.feishu.settings` are rejected (fail-closed): put
   `{"wat":1}` in settings and Start refuses to launch.

## Confirming it can hear / speak

1. Create a custom application with bot capability in the Feishu Open Platform,
   configure `FEISHU_APP_ID` / `FEISHU_APP_SECRET`, and set
   `allow_from: ["feishu:<your open_id>"]` (the open_id can come from the first
   event log or the contacts API).
2. Use a personal account to send the bot a direct-message text → the Vivy log gets a
   Journal record for the inbound event (channel=feishu, sender=feishu:ou_...) → after
   the run completes, the bot replies with plain text in the same p2p conversation
   (`im.v1.messages`, `receive_id_type=chat_id`).
3. @ the bot in a group → **no response** (group chat is outside the first slice).
4. Change `FEISHU_APP_SECRET` to an incorrect value and start → Start fails
   closed immediately (the gateway rejects the first connection), without leaving a
   "deaf ear" pretending to be online.
5. Disconnect the network for a few minutes and restore it → the supervisor redials
   with a new client and send/receive self-heals; after Stop/restart there is no revival
   or leak from the old connection (`-race` all green).

## International edition

`settings.is_lark: true` + international Lark app credentials → the same flow
uses the `open.larksuite.com` domain.

## Known boundaries (by design)

- Direct-message plain text only; group chat, cards, images, and reaction replies do not
  enter or leave.
- The lark SDK dependency tree does not support the 386 target (compilation failure is
  a hard constraint).
