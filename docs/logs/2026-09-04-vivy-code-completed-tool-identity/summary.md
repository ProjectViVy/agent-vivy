# VIVY CODE completed projection and tool identity

## Shipped

- Added explicit `model.completed` interpretation shared by the built-in fullscreen TUI, packed fullscreen face, and plain REPL. Completion content is authoritative for its model round, including an explicitly empty result.
- Added model-round fences for `model.request`, reasoning, tools, gates, and terminal events. A provider response that combines assistant preamble text with tool calls now flushes that text before `tool.requested` and clears the pending accumulator.
- Preserved `tool_call_id` through notices, approval gates, tool cards, and history reconstruction. Non-empty IDs match exactly; tool-name fallback is limited to legacy events with no ID.
- Added shared reducer tests plus built-in and packed wire-level tests for completed-only output and concurrent same-name tool calls.

## Explicitly not done

- `TUI-STREAM-N4` remains open: a full `model.completed.content` can exceed the nominal single-event payload limit. This delivery preserves existing Journal/history compatibility and does not silently truncate content or change the wire schema.
- `TUI-STREAM-N6` remains open: tool-call preamble deltas are durable and visible during stream/replay, but the existing Message-store projection does not retain them after terminal session-history reload. An extra completion was deliberately rejected because it corrupts request/trajectory pairing.
- Browser UI completed-only rendering, split diff, and missing sidebar model/cost/workspace truth are outside this VIVY CODE TUI slice.
- The linear REPL cannot retract bytes already emitted when a provider sends a completion that contradicts its deltas; it displays the authoritative replacement on a new line. Fullscreen projection replaces the bubble in place.
