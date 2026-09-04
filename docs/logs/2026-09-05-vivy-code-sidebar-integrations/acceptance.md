# Acceptance

1. Start VIVY CODE in a terminal at least 120 columns wide and 30 rows tall.
2. Confirm the right rail contains the active session/workspace data and, when configured, an `MCP` section plus `Skills · enabled`.
3. Confirm an MCP server that has not completed a handshake says `configured`; run `/mcp <server>` and confirm a successful probe refreshes it to `initialized` without restarting the TUI.
4. Confirm disabled skills are absent and enabled skills are listed. No row should claim `mounted`, `loaded`, or `effective` session state.
5. Add enough modified files/integrations to overflow the rail. Click inside the rail and use the wheel: only sidebar content moves while the `VIVY CODE` logo remains fixed.
6. Click the chat/editor area and confirm wheel input no longer moves the sidebar. Open Sessions, command palette, file completion, or a gate and confirm wheel input cannot leak to the obscured rail.
7. Resize below the wide breakpoint and confirm hidden sidebar focus/offset are cleared.
