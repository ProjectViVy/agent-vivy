# VCP-D3 — GUI chat-level compact control

## What landed

Manual compaction is now reachable from the chat composer, not only the
Settings card.

- **Control**: a `Shrink` button next to the context-usage ring in the
  composer bottom row (visible only when `compaction_enabled`). It opens a
  popover with an optional instructions field and "Compact now", calling
  the D1-plumbed `compactSession(sessionId, instructions?)` →
  `context/compact` RPC.
- **Busy semantics**: the trigger stays enabled while a run is active
  (title explains the run must finish); a rejected call (409/busy) is
  surfaced through the composer's notice slot, not silently dropped.
- **Feedback**: success shows `Compacted X → Y tokens`; `skipped` results
  show "Nothing to compact yet".
- **Transcript fold marker**: `context.compacted` already rendered a
  notice row; `auto_retry.started`/`auto_retry.finished`/`provider.retry`
  (VCP-D2 events) now also fold into notice rows so the GUI displays
  overflow recovery inline. `RunRowNotice` gained a `tag` field
  (`compact` renders the i18n compaction label, `retry` renders raw).
- i18n en+zh for all new strings.

## Boundary kept

No auto-compaction UX changes — the Settings card keeps policy knobs.
