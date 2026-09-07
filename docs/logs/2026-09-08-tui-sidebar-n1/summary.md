# TUI sidebar N1 wave 1

Date: 2026-09-08

Wave 1 carries authoritative MCP status and enabled-skill origin from the
runtime/RPC boundary through the shared TUI surface, live mapper, and right
rail renderer.

- MCP rows now preserve handshake error, dynamic missing-auth state, and the
  last successful projected tool count (`-1` means unknown). The renderer
  keeps the main state row and bounded detail rows separate.
- Skill rows show the backend-provided catalog origin when present. Origin is
  availability provenance only; it is not evidence that a skill was mounted
  into a session.
- MCP status records remain owned by `MCPBackend.ServerStatuses()`. Retired
  same-named configurations cannot write into a replacement's status record.

Eino capability check: the pinned `github.com/cloudwego/eino-ext/components/tool/mcp`
v0.0.9 surface provides `GetTools(ctx, *mcp.Config)` and delegates catalog
enumeration through its `client.MCPClient.ListTools`; Eino v0.9.13 supplies the
`tool.BaseTool.Info`/schema projection. It has no sidebar health, auth-env, or
status API, so the thin runtime adapter remains the correct custom seam while
Eino stays behind the runtime import quarantine.

Explicitly not delivered: MCP event/notification status, true
skill-to-session mount provenance, LSP bounded failed/exited history, workspace
diagnostics aggregates, and `TUI-PROJECTION-ORDER`.
