# Eino-native MCP migration

日期：2026-09-06

## 已完成

- 删除 `internal/runtime/mcp_backend.go` 中自研 JSON-RPC、HTTP、SSE、session、request-id 与协议解析；改用 `mark3labs/mcp-go v1.0.0` 官方 `client.NewStreamableHttpClient`，以 `LATEST_PROTOCOL_VERSION` 首选 modern discover，并让官方 client 对旧 server 回退 legacy initialize。
- 通过 `eino-ext/components/tool/mcp v0.0.9` 的 `GetTools` 完成官方 MCP tools/schema 转换，但只投影到 Vivy 不可信 catalog，不把 Eino tool 直接挂到 model。
- 保留 `mcp_list_tools`、`mcp_call`、`PrepareMCPCall` 审批门、`MCPServerConfig`、status/catalog/ReplaceServers 与 resources/prompts 产品 contract。
- 保留 8s operation timeout、512KiB raw response guard、256KiB content/catalog budget、32 页上限、重复 cursor 检测、browser-use 排除、确定性 server order 与 fail-closed projection。
- bearer header 每次 HTTP request 读取 `AuthEnv`；幂等 list/read/get 在 session terminated 时最多重建并重试一次，tools/call 不重试；首次并发调用共享一次初始化握手。
- Replace/remove、failed initialize、session replacement 与 App shutdown 均关闭官方 client；被替换 client 的退休清理由 backend 跟踪并由 Close 等待，多 client close 并发，避免 N×5s 串行阻塞。
- 删除 `internal/tools/mcp.go` 中已无消费者的四个旧 alias/type。

## 明确未做

- 未增加 stdio、OAuth 或 continuous listening。
- 未改变 settings/RPC/UI/TUI contract、Journal、Policy/HITL 或远程工具治理路径。
- 未接入 Eino 直接 tool mount；Eino 当前只覆盖 tools，且会将 `CallToolResult.IsError` 转为 Go error，因此 resources/prompts/lifecycle 与 Vivy `isError` 仍使用同一 mcp-go typed client。
- 未关闭 EINO-BOUNDARY-AUDIT track 的 tool_search、Sequential Thinking、plantask compatibility、stream observer 等其他候选。

许可证口径：Eino MCP component 为 Apache-2.0；mcp-go 为 MIT。
