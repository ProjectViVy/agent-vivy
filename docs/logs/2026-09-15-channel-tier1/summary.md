# Channel tier-1: 2026-09-15

## What shipped

Three channel features on top of the channel-hardening batch, one focused
commit each on `feat/channel-tier1`:

1. **Failed-delivery list + redeliver** (`c773a66`). `channel_deliveries`
   rows parked as `failed` were invisible and unactionable. Now:
   `ListFailedChannelDeliveries` on both backends (conformance CN-29 pins
   the listing partition), `Host.FailedDeliveries` / `Host.RedeliverDelivery`
   (reuses the enqueueDelivery draining gate and the markPending-before-spawn
   crash ordering), RPC `channel/deliveries/list` + `channel/deliveries/redeliver`,
   and a Settings UI section with a per-row redeliver button. Ruled: the
   accumulated attempt count is kept, so with a spent budget one click is
   exactly one send attempt — no retry loops.

2. **Health probing + error classification, CH-R-1 closed** (`291a45e`).
   `plugin.ErrorClass` (`rate-limit | temporary | dead`) + `plugin.HealthError`
   land in `sdk/port/channel`; `HealthChecker.Health` is contracted as
   read-only internal state (no network I/O). All five adapters implement it
   from their supervised-transport state; the Host probes started adapters
   inline and surfaces `health {ok, class, detail}` on channel/inspect and
   the Settings UI (cards + editor badge). Unclassified errors default to
   `temporary`. Contract §8 Reliability row + §1 Decision Record updated.

3. **HITL notifications + channel approval commands** (this batch). A channel
   run suspended for approval notifies its originating chat (one plain text:
   tool name, session pointer, command hint) — a direct adapter Send that
   never touches the delivery ledger. The allow-listed sender can answer
   `/approve [id]`, `/deny [id]`, `/pending`: commands are journaled like any
   inbound message but open no run, track no target, record no intent.
   Decisions forward through the new `Service.DecideApprovalAsActor` with
   actor `channel:<channel>:<sender>` — the kernel's single decision path,
   session-scoped (a chat can never see or decide another chat's approvals).
   This supersedes, for this narrow surface, the earlier "channel user = ACP
   remote principal" reading; the ruling and its boundaries are recorded in
   contract §1/§12. HITL-P1-6 (external notifications, channel slice) closed.

Plus `docs/research/2026-09-15-channel-native-approval-ui.md` (why native
approval cards are out of this generation) and the P9 conformance evidence
refresh each commit required.

## Explicitly not done

- Native approval cards (Feishu interactive cards, DingTalk STREAM cards,
  QQ button templates) — tracked as `CH-NATIVE-CARD`; blocked by QQ template
  permission + 5-minute group passive window and by the text-only outbound
  contract.
- QQ C2C passive replies expire after ~60 minutes: a much longer approval
  becomes unreachable from QQ even by text (local UI stays authoritative).
- `user.question_required` notifications (same shape as approvals, separate
  slice), TUI channel surface, webhook/listen inbound, media/groups/stream,
  wecom, CH-C8/CH-C9.
- Redeliver attempts reset (ruled against: cumulative budget kept).
