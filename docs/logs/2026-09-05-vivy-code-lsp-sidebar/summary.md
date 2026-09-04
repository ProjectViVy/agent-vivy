# VIVY CODE workspace-scoped LSP sidebar

## Delivered

- Added an optional, typed `LanguageServerStatusProvider` to the public plugin SDK. It exposes only logical language plus `starting`/`initialized`; no PID, command, arguments, absolute path, stderr, environment, or diagnostic body crosses the boundary.
- Made the LSP plugin manager the sole status owner. Snapshots filter by exact workspace and never spawn, probe, revive, or otherwise mutate a language server.
- Removed the manager's global-lock-during-initialize flaw. Concurrent callers for one language/workspace now share one in-flight start while unrelated status reads and other keys remain responsive.
- Added a read-only `WorkspaceManager.Existing` resolver so status inspection cannot create a run workspace.
- The composition root maps `session -> latest primary run (SQL LIMIT 1) -> existing workspace -> packed tool-world status owner`. Child or historical workspaces cannot leak into the current row. Provider fan-out, returned rows, in-flight calls, and wall time are all bounded; timeout/panic becomes unknown instead of failing the whole sidebar.
- Extended `session/sidebar`, shared TUI surface, built-in live face, and packed live face with the same optional LSP snapshot. No packed owner means the whole section stays hidden; a known idle owner says `None initialized`.
- Successful `lsp_*` tool completion coalesces one sidebar refresh, while the existing boot/session-switch/run-terminal refresh points remain authoritative. Once an owner is known, a five-second TTL refresh removes exited/reaped process rows without per-frame probing.
- Control-plane labels are ANSI/OSC stripped, length bounded, and restricted to logical language identifiers; path-, command-, control-, and bidi-shaped values fail closed. Unknown states are filtered at RPC, both face mappers, and the renderer.

## Explicitly not delivered

- Hard-coded language commands are not presented as configured servers. A row exists only while the plugin manager owns a starting or initialized process.
- Failed/exited history is not retained by the current manager, so the UI does not invent `error` or `stopped` states.
- Workspace-wide diagnostic counts are not available. A single file's diagnostics are not misrepresented as project totals.
- Default generations without the LSP plugin expose no LSP section. Packing the plugin is required.

Bounded failure/exited history and authoritative workspace diagnostic aggregates remain in `docs/TODO.md`.
