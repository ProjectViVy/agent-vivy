# VIVY CODE sidebar integrations and mouse

## Delivered

- Added a secret-free MCP backend status snapshot. The sidebar reports an enabled runtime server as `configured` until its lazy MCP handshake succeeds, then as `initialized`; it never probes the network merely to draw UI.
- Added enabled-skill catalog rows from the same backend used by the runtime skill middleware. The label deliberately says `Skills · enabled` and does not claim that a skill was loaded or mounted in the current session.
- Extended `session/sidebar`, the shared TUI surface, built-in `internal/tui`, and packed `faces/tui` with the same MCP and skill projection.
- Refreshes sidebar truth after successful `/mcp` inspection as well as the existing boot, session-switch, and run-terminal refresh points.
- Enabled Bubble Tea cell-motion mouse input in both fullscreen launch paths. A left click selects the scrollable sidebar, wheel presses scroll it in bounded three-line steps, clicks outside release focus, and open dialogs/gates block background scrolling.
- Split the sidebar into a fixed two-line VIVY CODE logo and an independently scrolling content viewport.

## Explicitly not delivered

- LSP health remains open. The LSP manager is private to an optional plugin and keyed by language plus run workspace; there is no current session-to-workspace status port. Reporting process-wide connections would mix sessions, and the default generation may not contain the plugin.
- MCP error/auth/count/event states remain open. The current backend only owns configured catalog and cached handshake truth.
- Skills are not described as session-mounted. Current journal mount payloads do not retain skill-name/hash provenance, so enabled catalog is the strongest honest projection.
- Main-chat history still has no independent viewport, so mouse wheel outside the selected sidebar remains a no-op.
