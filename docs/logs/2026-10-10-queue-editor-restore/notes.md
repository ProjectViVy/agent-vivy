# Editor recovery design

Queue admission and removal remain server-owned. A successful recall returns one
complete `QueuedTurn`; the adapter keeps captured image/file bytes and continuity
selectors together while the surface renders safe attachment metadata and allows
text/thinking/mode edits. Resend uses the captured bytes instead of re-resolving
paths. The GUI shows captured images; the TUI retains raw bytes privately and
exposes only safe metadata under its existing Surface contract.

Aborted or cleared turns are separate editor recovery drafts. They are never
automatically admitted, merged across differing options, or driven by a face-local
scheduler. Alt+Up recalls them one at a time. Existing request IDs and selected
history scopes remain stable through failed resends. Unsupported recovery must
leave the durable turn pending rather than acknowledge removal and discard data.

The parent owns integrated product CI and browser end-to-end checks. This lane
owns focused GUI/TUI adapter tests and records their actual results separately.
