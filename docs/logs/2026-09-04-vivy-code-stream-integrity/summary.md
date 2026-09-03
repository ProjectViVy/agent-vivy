# Vivy Code stream integrity

## Changed

- Pure provider reasoning chunks no longer emit empty `model.delta` events.
- Both the built-in and packed TUI ignore historical empty deltas instead of
  treating them as the boundary between reasoning and the final answer.
- The built-in TUI notification ingress now uses the same mutex-protected,
  ordered FIFO shape as the packed face. A stalled 40 ms render tick can no
  longer silently discard notices after a fixed 128-event buffer fills.
- Both TUI transports track durable event sequence numbers. A gap triggers a
  new `run/subscribe` from the last contiguous sequence, replay duplicates are
  suppressed, unrendered event types still advance the cursor, and notices
  beyond a gap remain buffered until the missing replay arrives.
- Replay subscriptions are single-flight. A transient replay error preserves
  the active run and retries with a one-second backoff instead of cancelling
  the user's turn.
- Run ownership is installed before the initial subscription, removing the
  race where replayed events could arrive before the new run became active.
- Regression coverage preserves Chinese text, emoji, newlines, the
  reasoning-to-answer boundary, burst delivery, sequence gaps, and replay
  duplicates.

## Scope

This delivery changes Vivy and its packed TUI face only. It does not touch
Vivy Studio or any tenant Journal. Shared-driver consolidation remains a later
wave; this cut fixes the confirmed local silent-drop, empty-delta split, and
durable stream-gap paths in both existing drivers.

The remaining plain-REPL cursor parity and explicit unsubscribe/bounded-buffer
consolidation are recorded as `TUI-STREAM-N1` and `TUI-STREAM-N2` in
`docs/TODO.md`; they are not silently treated as complete.
