# VCP B2 — TUI steering and follow-up key tracks

Date: 2026-10-06. Story: `docs/superpowers/plans/vivy-code-parity/B2-tui-steering.md`.

## What shipped

The TUI now drives the B1 dual-track kernel queue with the pi key scheme:

- **Enter while busy** → `turn/steer` on the kernel queue; the agent resume
  leg sees the text and the composer shows an optimistic user bubble (the
  HistoryModifier resume path emits no user event, so the face echoes it).
- **Alt+Enter** (or **Ctrl+Q** where terminals cannot send Alt+Enter) →
  `turn/follow_up`; lands after the run settles via the settle-admission
  poll (`queue/state{after_run_id}`).
- **Alt+Up** → `queue/dequeue` (new RPC verb + `Service.Dequeue`): LIFO-pops
  the newest pending follow-up and restores its text into the editor via
  `surface.RestoreInputMsg`.
- Queue display shows **both lanes**: the meta line carries
  `SteerQueued`/`FollowUpQueued` counts (i18n `vivy.tui.live.queueLanes`),
  and `/queue` renders previews of steering + follow-up items.
  `/queue clear` calls `queue/clear` and restores the newest text locally.
- **Abort/Esc** flushes the kernel queue; `turn.dequeued` payloads now carry
  `text` so the face restores the newest item into the editor (pi behavior).

Named actions now (configurable under G2 later): `submitFollowUp` and
`Dequeue` are driver-level verbs (`surface.Driver` gained
`SendFollowUp`/`Dequeue`), not buried key-maps.

Idle Enter is unchanged (starts a run; `queueStartFallback` covers the
wire when a queued verb lands on an idle session).

## Mechanism notes

- `payloadTurnDequeued` gained `text` — the only way a face can restore
  text without tracking queue ids; kernel journals it for every dequeue
  reason except `started`.
- Live controller keeps a local FIFO **only** for attachments/contextPaths
  (kernel queue is text-only per B1 design).
- `stream.Notice` gained `QueueTrack`/`QueueReason`; `Interpret` maps
  `turn.queued|dequeued|steered` → `queue_*` kinds carrying payload text.
- Session load sets `queueDirty` so a rebuilt kernel queue (restart
  recovery) refreshes lane counts on the next tick.
- sdk cannot import `internal/domain` → literal `"steer"`/`"follow_up"`
  strings in `sdk/tui/live/controller.go`.
