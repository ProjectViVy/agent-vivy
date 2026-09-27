# 2026-09-15 — Channel Text Loop (tier-1: typing / markdown / reply threading / group triggers)

## What changed

1. **Rebased the lane onto gate-0** (`63f9ea7` capability forwarding, `1cdd60b` rune ceilings, `2964491` fence-aware splitRunes) and resolved the semantic conflicts with the parallel health (CH-R-1) and approval commits; P9 conformance evidence re-pinned to the combined tree (`test: refresh P9 conformance evidence after the gate-0 rebase`, `a86b063`).
2. **Contract first** (`docs(channel): rule the tier-1 text loop`): §1 Decision Record row — group triggers are mention-only (fixed, no config knob) and outbound markdown scope per ear; §12/§14/§19 first-cut exclusions updated; §7 outbound duty records the first-chunk reply rule; §14.3 per-package scope rewritten. Amended once at wrap-up when the Feishu implementation revealed the `MentionedType=="bot"` trigger shape.
3. **Typing** (`feat(channel): typing indicator as a host-owned live surface`): the port's `Typing(ctx, chatID)` face finally has a caller. ChannelHost runs one loop per accepted run (immediate ping + 4 s cadence, 5-minute cap) started at target registration and ended by any terminal event, StopAll, or the first platform failure. Never journaled, no event types, no config knob. Adapters: telegram `sendChatAction`, discord `ChannelTyping` (new seam method), qq `InputNotify` (msg_type 6) anchored to the open passive window; dingtalk/feishu have no platform API and honestly stay false. The gate-0 record left call-path forwarding to this batch: `CapabilitySource` now also carries live calls (typing, health), and `probeHealth` follows the same seam — repairing production health probing, which discovery-only forwarding could never see.
4. **Markdown** (`feat(channel): outbound markdown with plain-text fallback`): telegram converts model markdown to the HTML subset (picoclaw approach rewritten read-only: code/links/URLs lifted before escaping) and resends a rejected formatted body as plain text; dingtalk posts the robot markdown shape to the sessionWebhook with errcode-driven fallback; qq grows the opt-in `markdown` settings flag (default off) with the plain retry; discord renders natively (untouched); feishu stays plain text.
5. **Reply threading** (`feat(channel): reply threading to the triggering message`): the host records the triggering inbound message id on the outbound target and quotes it via `OutboundMessage.ReplyTo` on the first chunk only. telegram `ReplyParameters` (with `allow_sending_without_reply`), discord `MessageReference` (new `ChannelMessageSendComplex` seam method), qq passive anchor = `ReplyTo` msg_id (seq discipline unchanged). In-process only: the durable intent row carries no message id, so restart-recovered redeliveries send unthreaded.
6. **Group triggers** (`feat(channel): mention-only group triggers for all five ears`): every adapter normalizes its own group shape at the same seam that used to hard-drop non-private conversations. discord gates guild messages on a Mentions hit for the READY-state identity; dingtalk on `IsInAtList`; feishu on a mention entry typed `bot`; telegram on mention/`text_mention`/`/cmd@bot` entities matched against the getMe identity (forum topics fill `TopicID`, replies set `MessageThreadID`); qq decodes `GROUP_AT_MESSAGE_CREATE` with its own ws dispatcher into a local `group_openid`/`author.member_openid` struct — bypassing the botgo v0.2.1 `group_id` defect without a dependency upgrade (gateway intent now includes the group bit; sends/typing route to `/v2/groups/{group_openid}/messages`). allow_from semantics unchanged; group chats get independent sessions through the channel+chat+topic key.

## Scope

- `internal/channelhost/` (typing loop, call-path resolution, reply target), `internal/app/channels.go` (live capability disclosure), `sdk/port/channel/channel.go` (CapabilitySource doc amendment only — no port surface change), `plugins/{telegram,discord,qq,dingtalk,feishu}/`.
- Contract: `docs/architecture/VIVY-CHANNEL-PACK.md` (§1 row, §7 duty 4, §12 first-cut sentence, §14/§14.3, §19); P9 evidence files.

## Explicitly not done

- Media in/out, edit/delete/reaction/placeholder, incremental streaming edits (later tiers; tier-2 owns media).
- Feishu typing / outbound markdown / reply threading (no platform typing; no native markdown message type; quote semantics are inbound-context — separate item).
- QQ markdown default-on (opt-in flag; most robots lack the markdown permission).
- Group prefix/permissive modes (mention-only is fixed this cut; a config knob needs its own decision record).
- Persisting the reply anchor across restarts (needs a `channel_deliveries` migration for a cosmetic header).
- Native approval cards (CH-NATIVE-CARD stays open).
- Feishu bot-open_id-precise mention matching (the pinned lark SDK has no self-bot-info endpoint; matching is by mention type — a mention of ANOTHER bot also triggers, see acceptance.md).
