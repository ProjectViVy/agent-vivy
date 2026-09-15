# 2026-09-15 — channel interact (edit/delete, placeholder, feishu ack/card)

## What changed

Batch 3 of the channel arc: the interaction capabilities, delivered on
`feat/channel-interact` in five focused commits plus the board/log.

1. **Contract first** (`9645131`): VIVY-CHANNEL-PACK.md §1 Decision Record
   row, §12 live-surface paragraph, §8 note, §14 prose, §14.3 scope rows.
   Port fixes at zero breakage (no implementer existed):
   `EditMessage` gains the chat id, `React` returns the platform reaction
   id, `ReactionRemover` completes the withdrawal half, and Discover
   advertises `Reaction` only when both faces exist.
2. **telegram** (`d967e8f`): `EditMessage` (markdown→HTML + plain-text
   fallback; a `message is not modified` answer is success — the
   idempotency guard), `DeleteMessage`, `Placeholder` (fixed plain
   "Thinking…" copy, returns the message id).
3. **discord** (`3a0da28`): the session seam grows
   `ChannelMessageEditComplex`/`ChannelMessageDelete`; edit/delete/
   placeholder all ride the never-opened REST send client, never an ear
   session.
4. **feishu** (`cb3220d`): every outbound text part renders as a schema-2.0
   markdown interactive card; platform error 11310 falls back to plain
   text (typed `feishuAPIError`, no string parsing); edit via card `Patch`
   (the only Patch payload Feishu accepts), delete, card placeholder, and
   the ack reaction — a random emoji_type from `settings.ack_emojis`
   (default `THUMBSUP`, explicit empty list disables) whose returned
   reaction id `RemoveReaction` withdraws.
5. **Host live surface** (`bf22613`): the placeholder/reaction half of the
   live surface on the typing precedent's shape — a per-run entry on the
   delivery target, one goroutine sending the faces best-effort (bounded
   calls) then parking until the turn settles or the 10-minute TTL fires.
   Settlement deletes the placeholder and withdraws the ack exactly once,
   for every terminal (completed/failed/cancelled), at StopAll, and at the
   TTL backstop. The reply itself stays a fresh durable send — the
   placeholder is never edited into the answer (the simple semantics the
   user ruled on). An approval-required event is not a terminal; the TTL
   covers the 5-minute approval expiry.
6. **Evidence** (`b6a1126`): P9 digests rotated (internal
   `98f4026e…→182bc905…`, telegram `01af8a2c…→32b8f1c2…`, discord
   `a78e7afc…→f109be76…`, feishu `987d1420…→78165b54…`; fixed points
   verified) and the capability pin flipped to the landed surface.

## Scope decisions (user-ruled)

- Placeholder keeps the **simple semantics**: delete at any terminal,
  reply always sent fresh through the durable delivery path. The
  picoclaw-style "edit the placeholder into the answer" was rejected for
  this generation: it would trade the at-least-once delivery narrative for
  platform-edit luck.
- Placeholder TTL backstop: **10 minutes** (covers the 5-minute approval
  expiry; in-code precedents bracket it at 30s delivery timeout and 5m
  approval expiration).
- feishu outbound: **all** text goes card-first (one code path), not a
  markdown-detection heuristic.

## What was explicitly not done

- **3b (gated, registered as `CH-INTERACT-3B` in `docs/TODO.md` §0.1)**:
  streaming drafts (`StreamingCapable` incremental edits, telegram
  `sendMessageDraft`-style) and command menus (`SetMyCommands`). Both are
  contract-gated; they start only when the user names the item.
- **qq / dingtalk interaction**: qq has no edit or reaction API and its
  passive ids live only inside the 60-minute reply window; dingtalk's
  sessionWebhook returns no message id. Recorded in §1/§14.3, nothing
  implemented.
- **UI**: none. The RPC inspect surface already projects the capability
  flags, and the channels settings form writes only `enabled`/`allow_from`
  (pointer semantics), so the new `ack_emojis` settings key cannot be
  clobbered by a UI save; it is set through the raw settings JSON.
- Placeholder text is a fixed adapter constant, not a settings knob.
- Forum-topic placeholders (telegram) land in the topic's General: the
  port's `Placeholder(ctx, chatID)` cannot address a topic. Recorded wart.
- A process death orphans an already-sent placeholder (the live surface is
  in-memory by ruling; §12 documents the lapse), the same trade typing
  made.

## Base and landing

Developed in the `agent-vivy-channel-interact` worktree off
`feat/channel-tier1` at `029f6cc` while the tier-1/tier-2 lane kept
landing in the root tree (text loop, inbound/outbound media). Landing back
rebases onto that lane's settled tip and re-runs the gate.
