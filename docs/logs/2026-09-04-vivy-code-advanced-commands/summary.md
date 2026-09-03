# Summary

This delivery closes the advanced VIVY CODE command gap across the shared
fullscreen view, the built-in live driver, the packed `faces/tui` driver, and
the legacy line REPL.

- Added real control-plane commands: `/compact`, `/fork`, `/rewind`,
  `/todos` (`/tasks`), `/stats`, `/skills`, `/mcp`, `/files`, and `/tools`.
- Added one shared argument contract and deterministic JSON result renderer.
- Added explicit confirmation and in-flight/session-epoch fences for context
  mutations. Successful compact/rewind/fork operations refresh or switch the
  active session from authoritative RPC state.
- Kept data claims truthful: stats are labelled aggregate, skills are catalog
  data, MCP output is configured catalog or one-shot probe data, and a
  no-op compaction is labelled skipped.
- Reserved `!` and `@` safely: unsupported single prefixes fail locally and
  never reach the model or host shell; `!!` and `@@` escape one prefix into
  ordinary model text.

Not done: a governed shell/reference RPC does not yet exist, so `!` and `@`
execution remains unavailable and is tracked as `TUI-CMD-N2`. Model/thinking
selectors, image attachment entry, and richer sidebar truth remain tracked by
the existing `TUI-PARITY-2` and `TUI-SIDEBAR-N1` rows.
