# VIVY CODE live-stream backpressure

## Delivered

- Added a model-stream observer at the raw ChatModel boundary, before Eino
  materializes the stream. Provider chunks now receive immediate mapping,
  budget accounting, Journal persistence, and Bus publish; the later Eino
  event still supplies usage and tool-call metadata without duplicating text.
- Installed the same observer on approval/question resume and added a tool
  barrier so the next model turn cannot overtake its preceding durable
  `tool.started`/`tool.finished` batch.
- Added cancellation-aware blocking notifications for durable `run/event`
  traffic. A full bounded peer writer queue now applies backpressure instead
  of silently ending the run subscription; ordinary notifications retain
  their fail-fast overload contract.
- Bound subscription lifetime to peer shutdown even while no run event is in
  flight, and made RPC responses wait for bounded writer capacity so recovery
  responses cannot be displaced by the stream they control. Parent-context
  cancellation, pre-response peer close, and abandoned after-response
  callbacks are cleaned up as one lifecycle.
- Split oversized reasoning and answer deltas into bounded events without
  discarding Unicode text, and reject payload limits too small to encode the
  event envelope safely.
- Replaced whitespace-normalizing TUI wrapping with ANSI-safe,
  grapheme-aware cell wrapping. Chinese text, emoji clusters, internal spaces,
  and newlines retain their intended display, including whitespace crossing a
  wrap boundary, while terminal controls are stripped or normalized.

## Scope

This is the end-to-end Vivy kernel/RPC/shared-TUI correction for live stream
continuity. It does not change Studio or tenant data. Plain REPL durable cursor
parity and explicit subscription/inbox lifecycle remain the separately tracked
`TUI-STREAM-N1` and `TUI-STREAM-N2` follow-ups.
