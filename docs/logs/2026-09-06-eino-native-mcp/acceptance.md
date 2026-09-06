# Acceptance

1. 配置两个 Streamable HTTP MCP servers 后，`mcp_list_tools` 按 server/name 稳定返回不可信 catalog；browser-use 工具不出现，schema 来自 Eino `GetTools`。
2. `mcp_call` 仍只能经 Vivy tool adapter 与 `PrepareMCPCall` 审批路径到达远端；远端 `isError` 保留在 `MCPCallResponse`，不会被 Eino 的 Go error 语义覆盖。
3. resources/list/read 与 prompts/list/get 可在同一 session 工作，保留 title/size/arguments/metadata；非文本 prompt、未知 capability、超限内容均 fail closed。
4. 替换或移除 settings 中的 server 后，旧 client 最终关闭；应用 shutdown 会关闭全部 MCP clients，且 close 不按 server 串行等待 5 秒。
5. 修改 `AuthEnv` 对应 token 后无需重启，后续 HTTP request 使用新 bearer；首次并发操作只发送一次 initialize。
6. 人工验收可先运行本目录 verification 中的 MCP tests 与 `just ci`；再以 mcp-go `examples/everything` 的 HTTP 模式作为本地 server，在 split UI 的 `/mcp` 页面新增并探测它，应显示“已连接 · 6 个工具”。测试与 smoke 都不读写 `data/vivy.db`、`data/demo` 或 `data/workspaces`。
