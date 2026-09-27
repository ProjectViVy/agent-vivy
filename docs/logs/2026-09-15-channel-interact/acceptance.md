# Acceptance — channel interact

How a human tells the batch works, once the ears are configured with real
tokens.

## Placeholder (telegram / discord / feishu)

1. Send a message to the bot in an allow-listed chat.
2. Immediately a "Thinking…" message appears (telegram/discord: a plain
   text message; feishu: a card).
3. When Vivy answers, the placeholder disappears and the reply arrives as
   a fresh message — on every outcome. Ask something that fails or cancel
   the run: the placeholder still disappears, with no reply after it.
4. A pending approval keeps the placeholder visible for the whole wait
   (well past the 5-minute approval expiry, inside the 10-minute TTL).
5. Stop the process (or `StopAll`) while a run is live: the placeholder is
   cleaned up on the way down.

## feishu specifics

6. Every reply arrives as a formatted card (markdown renders — headings,
   bold, lists, code), not plain text.
7. On receiving a message the bot adds a small emoji reaction (default
   THUMBSUP, or one drawn from `channels.feishu.settings.ack_emojis`) to
   the sender's message; when the turn settles the reaction is gone.
8. Setting `ack_emojis: []` disables the reaction entirely.
9. A gigantic reply that breaks the card limit still arrives — as plain
   text instead of a card.

## Edit / delete (telegram / discord / feishu)

10. The capabilities show up in the settings inspect surface: telegram and
    discord advertise Edit/Delete/Placeholder (plus Typing/Health),
    feishu advertises Edit/Delete/Reaction/Placeholder; qq and dingtalk
    advertise nothing new — by ruling, since their platforms give nothing
    to anchor (§1/§14.3).
11. The edit face is idempotent on telegram: re-running the same edit does
    not error ("message is not modified" is success).

## Trust markers

- The Journal and the delivery ledger carry no placeholder or reaction
  rows (asserted by tests; an operator inspecting `channel_deliveries`
  sees only the reply intents).
- No approval decision can be expressed through a placeholder or a card —
  the HITL command surface is unchanged.

## Known limitations (accepted, recorded)

- A hard process kill between "placeholder sent" and the next start leaves
  that one placeholder message on the platform (the live surface is
  in-memory by ruling; the §12 ledger paragraph documents the lapse).
- A telegram forum-topic turn's placeholder appears in the topic's General
  (the port face cannot address a topic).
