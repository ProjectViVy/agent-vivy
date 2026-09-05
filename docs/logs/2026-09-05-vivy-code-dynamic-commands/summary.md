# VIVY CODE dynamic commands

## Shipped

- Added an explicit `user-invocable` Skill frontmatter field and preserved it through catalog reads and enable/disable rewrites.
- Added typed `commands/list` and `commands/expand` control-plane methods. The catalog combines enabled user-invocable Skills with MCP prompts discovered through `prompts/list`.
- Added bounded MCP `prompts/list` pagination and `prompts/get` expansion. Initialize capabilities are retained so tools-only servers are skipped; only user-role text is joined, matching the Crush command contract, while other roles/content are ignored.
- Added dynamic slash rows to the shared fullscreen palette/help/direct parser and the plain REPL. Built-in and packed TUI clients use the same wire contract.
- Refresh the dynamic catalog whenever the fullscreen command palette opens. Failed refreshes preserve the last known catalog and render an explicit status.
- Preserve MCP argument metadata through both clients and open a fullscreen argument form from the palette. Required fields are validated locally before expansion; direct slash and REPL invocation retain the `NAME=value` form.
- Serialized asynchronous expansion with request, active-session, and opaque-command ID fences. Escape, expansion errors, empty results, and session changes cannot send stale text and preserve the original slash draft for retry.
- Reserved built-in commands cannot be shadowed. Stale or duplicated Skill/MCP identities, malformed MCP `NAME=value` arguments, unsafe controls, oversized metadata/input, and unsupported content fail closed.

## Scope

This delivery changes VIVY CODE and the first-party packed TUI face. It does not change Vivy Studio or tenant Journal data.

## Explicitly not done

- Arbitrary user markdown command directories are not introduced; installed Skills and configured MCP prompt servers are the authoritative dynamic sources.
- MCP prompt results are not executed locally and are not mounted as tools. They enter the existing governed turn path as model input only after explicit invocation.
- Review findings outside the four approved P1 fixes are deferred in `docs/TODO.md`; this cut does not add REPL live-refresh events, background expansion cancellation, or further async boot/editor restructuring.
