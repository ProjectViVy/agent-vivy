# Eino-native MCP migration

> Historical snapshot (2026-09-06), superseded by PLG-P4 closure on
> 2026-09-11. The retained `mcp_call`/`PrepareMCPCall` statements below record
> the pre-P4 model surface; current remote calls use MCPHost → ToolWorld →
> ToolHost and the former direct tool is not model-visible.

Date: 2026-09-06

## Completed

- Removed in-house JSON-RPC, HTTP, SSE, session, request-id, and protocol parsing from `internal/runtime/mcp_backend.go`; switched to the official `client.NewStreamableHttpClient` from `mark3labs/mcp-go v1.0.0`, using `LATEST_PROTOCOL_VERSION` as the preferred modern discover path and having the official client fall back to legacy initialize for old servers.
- Completed the official MCP tools/schema conversion through `GetTools` from `eino-ext/components/tool/mcp v0.0.9`, but projected it only into Vivy’s untrusted catalog rather than mounting the Eino tool directly on the model.
- Retained `mcp_list_tools`, `mcp_call`, the `PrepareMCPCall` approval gate, `MCPServerConfig`, status/catalog/ReplaceServers, and the resources/prompts product contract.
- Retained the 8s operation timeout, 512KiB raw response guard, 256KiB content/catalog budget, 32-page limit, duplicate-cursor detection, browser-use exclusion, deterministic server order, and fail-closed projection.
- The bearer header reads `AuthEnv` on every HTTP request; idempotent list/read/get operations rebuild and retry at most once when the session is terminated, while tools/call is not retried; the first concurrent calls share one initialization handshake.
- Replace/remove, failed initialize, session replacement, and App shutdown all close the official client; the backend tracks retirement cleanup for replaced clients and Close waits for it, while multi-client close runs concurrently to avoid N×5s of serial blocking.
- Removed the four obsolete alias/type definitions with no remaining consumers from `internal/tools/mcp.go`.

## Explicitly Not Done

- Did not add stdio, OAuth, or continuous listening.
- Did not change the settings/RPC/UI/TUI contract, Journal, Policy/HITL, or remote-tool governance path.
- Did not add a direct Eino tool mount; Eino currently covers only tools and converts `CallToolResult.IsError` to a Go error, so resources/prompts/lifecycle and Vivy `isError` still use the same mcp-go typed client.
- Did not close the other candidates on the EINO-BOUNDARY-AUDIT track, including tool_search, Sequential Thinking, plantask compatibility, and stream observer.

License position: the Eino MCP component is Apache-2.0; mcp-go is MIT.
