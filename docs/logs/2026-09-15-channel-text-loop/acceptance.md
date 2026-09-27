# Acceptance — Channel Text Loop (tier-1)

How a human can tell it worked, in product terms.

1. **Typing.** With a started telegram/discord/qq ear, send an allowed
   message that takes the bot a while to answer: the platform shows
   "typing…" within a second of your message and stops when the reply
   lands (or when the run fails — no eternal typist). On dingtalk and
   feishu nothing shows, and `channel/inspect` does not claim typing for
   them. Telegram, discord, and qq DO claim it: the channels settings
   page capability chips show 交互/Typing as advertised.
2. **Markdown.** Ask the bot something whose answer contains code blocks,
   bold, or links. On telegram the reply arrives formatted (code blocks
   boxed, bold bold); on dingtalk and qq (with `settings.markdown: true`
   for qq) the same. If a platform rejects the formatting, the reply still
   arrives — as plain text — instead of disappearing. Discord was already
   native; nothing changed there.
3. **Reply threading.** On telegram/discord/qq the bot's answer visually
   quotes your triggering message (reply header / reference); a long
   answer split into several messages threads only the first one. Feishu
   claims nothing. If the process restarts between the question and a
   recovered redelivery, that specific late reply may arrive without the
   quote header — it still arrives.
4. **Group triggers.** Add the bot to a group/chat (with the group
   members' sender ids in `allow_from`):
   - Talking about the bot without @-addressing it: silence.
   - @-mentioning it (telegram `@bot …` or `/cmd@bot`, dingtalk @-select,
     feishu @, qq group AT, discord @): it answers in the group, and the
     answer is not polluted with the @markup.
   - The group gets its own conversation context — what was said in a DM
     does not leak into the group session, and forum topics on telegram
     stay separate per topic.
   - QQ groups now work at all: the AT event is decoded by Vivy's own
     dispatcher, so no botgo upgrade was needed.
   - allow_from still governs: a sender not on the list is ignored even
     when @-mentioning the bot, and an empty allow_from still refuses to
     start the channel.
5. **Known limitation (recorded, not hidden).** On Feishu the group gate
   triggers on a mention typed `bot`; the pinned lark SDK cannot tell the
   bot's own open_id, so in a group that contains a second bot, @-ing that
   other bot would also wake Vivy. If that ever matters, the fix is a
   bot-info call (needs the SDK endpoint or a raw governed request) and
   its own decision-record amendment.

## What a human can check quickly

- `channel/inspect` (or the Settings page): advertised capabilities per
  ear — Typing on telegram/discord/qq only, Health on all five; health
  badges still show live transport state (the call-path fix keeps CH-R-1
  working through assembly wrappers).
- Group chat with two members, one in allow_from, one not: only the
  allowed sender's @mention gets an answer.
- `just ci` green on `feat/channel-tier1`, including the P9 conformance
  gate over the refreshed digests.
