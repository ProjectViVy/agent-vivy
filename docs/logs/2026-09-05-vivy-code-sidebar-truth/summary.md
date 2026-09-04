# VIVY CODE sidebar truth

## Delivered

- Replaced the right rail's session-collection emphasis with a Crush-shaped current-session/workspace projection.
- Added the typed `session/sidebar` control-plane snapshot for durable session activity, project root, active provider/model, reasoning capability, context, session usage/cost, and modified-file summaries.
- Added durable `sessions.updated_at` support to SQLite and Postgres, including bootstrap/upgrade migrations and activity-order session listing.
- Added a bounded `file_versions` read projection that reports net line additions/deletions without returning file bodies over RPC.
- Kept built-in `internal/tui` and packed `faces/tui` clients on the same shared surface and view, with stale asynchronous responses fenced by request generation and session ID.
- Added an independently scrollable sidebar entered with `Ctrl+Right`; ordinary text input and cursor movement remain owned by the editor.

## Explicitly not delivered

- LSP health, session-mounted MCP/skill status, and mouse-wheel sidebar input remain open because they do not yet have the required typed, authoritative control-plane contracts.
- Split diff and model selection remain under the broader TUI parity backlog.
