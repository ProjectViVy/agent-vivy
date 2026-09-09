# Acceptance

1. After configuring two Streamable HTTP MCP servers, `mcp_list_tools` stably returns an untrusted catalog ordered by server/name; browser-use tools do not appear, and the schema comes from Eino `GetTools`.
2. `mcp_call` can still reach the remote side only through the Vivy tool adapter and `PrepareMCPCall` approval path; the remote `isError` is retained in `MCPCallResponse` and is not overridden by Eino’s Go error semantics.
3. resources/list/read and prompts/list/get work in the same session, retaining title/size/arguments/metadata; non-text prompts, unknown capabilities, and over-limit content all fail closed.
4. After a server is replaced or removed in settings, the old client eventually closes; application shutdown closes all MCP clients, and close does not wait 5 seconds serially per server.
5. After changing the token corresponding to `AuthEnv`, no restart is needed; subsequent HTTP request operations use the new bearer, and the first concurrent operations send only one initialize.
6. For manual acceptance, first run the MCP tests in this directory’s verification and `just ci`; then use mcp-go `examples/everything` in HTTP mode as a local server, add and probe it from the split UI’s `/mcp` page, and verify it displays “Connected · 6 tools”. Tests and smoke runs do not read or write `data/vivy.db`, `data/demo`, or `data/workspaces`.
