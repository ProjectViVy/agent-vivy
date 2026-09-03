# Shared TUI stream core

## Changed

- Added `sdk/tui/surface` as the single public surface contract for sessions,
  messages, tool cards, gates, metadata, driver commands, and Bubble Tea
  messages. The built-in and packed face-local `surface` packages are now
  compatibility aliases.
- Added `sdk/tui/stream` with the normalized event decoder/notice mapper,
  lossless notification inbox, durable sequence cursor, ordering helper, and
  protocol-independent message/gate projection reducer.
- Converted `internal/tui` and `faces/tui` to thin transport adapters and
  lifecycle owners. Both now consume the same stream cursor, inbox, and
  reducer implementation.
- Added shared-core tests plus parity coverage through both TUI packages,
  including reasoning continuity, empty delta boundaries, unknown-event
  sequence advancement, gap replay, duplicate suppression, tool gates, and
  burst delivery.
- Updated the Face Pack and SDK index to make the boundary explicit:
  `sdk/plugin` remains the only Host-capability window, while first-party
  terminal faces may import authority-free `sdk/tui` presentation state.

## Explicitly not changed

- No Studio source, tenant journal, runtime data, or provider behavior was
  changed.
- Slash-command coverage, file/shell/image/MCP inputs, and Crush-style sidebar
  data are separate deliverables and remain outside this core extraction.
