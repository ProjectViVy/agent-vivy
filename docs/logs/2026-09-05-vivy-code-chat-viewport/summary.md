# VIVY CODE chat viewport

## Delivered

- Replaced the chat transcript's unconditional tail crop with an independent, bounded line viewport shared by built-in and packed fullscreen faces.
- Added follow-latest and paused-history states. `PgUp`/`PgDn`, `Home`, `End`, and the mouse wheel move history without new streamed content stealing a paused view; sending a turn resumes follow-latest.
- Matched the existing Crush focus contract: a sidebar click gives the wheel to the sidebar, while the non-sidebar owner is chat. Dialogs, file/command panels, and approval gates block mouse input from leaking to the hidden surface.
- Reset viewport ownership on active-session changes and when resizing makes the transcript non-scrollable.
- Made backspace remove a complete grapheme cluster and made truncation ANSI/cell aware, covering combining marks, ZWJ families, flags, skin-tone emoji, and CJK.
- Sanitized and cell-wrapped tool names/results before rendering. OSC, cursor controls, and bidi controls cannot be emitted from model/tool data, and tool cards fit even extreme narrow widths.
- Fixed packed-face initial prompts so a replayed boot result cannot schedule the same prompt twice.

## Explicitly not delivered

- The paused position is a rendered-line offset, not a durable message/segment anchor. Content growth above that offset can shift the visible semantic position.
- Full-history layout is recomputed during viewport clamping/help/render; a message-layout cache is deferred until profiling demonstrates a need.
- Session deletion still has an existing asynchronous ID/history loading boundary. A future loading epoch should make that transition explicit.
- Split approval diff presentation, model selection, and richer token/cost controls remain under `TUI-PARITY-2`.

These follow-ups are recorded in `docs/TODO.md` rather than hidden by this delivery.
