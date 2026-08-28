# MCP 面板接真实后端（2026-08-30）

## 变更内容

把 `/mcp` 从 `vivy.demo.mcp` 本地模拟改成真实管理面。面板上的添加 / 编辑 / 启用 / 删除 / 导入导出写入用户工作区 `settings.yaml`，并立刻进入 `mcp_list_tools` / `mcp_call` 的运行时目录。

### 内核

- `internal/app/settings`：`mcp_servers` overlay。nil = 沿用 `config.yaml` `runtime.mcp_servers`；非 nil（含空列表）整表覆盖。字段：`name` / `endpoint` / `auth_env` / `enabled`。`auth_env` 只接受环境变量名（D-010）。
- `applySettingsOverlay`：启用项叠到 `cfg.Runtime.MCPServers`。
- `EinoMCPBackend.ReplaceServers`：热替换目录并丢掉旧 session。
- Streamable HTTP SSE：`text/event-stream` 响应解析 `data:` 行中的 JSON-RPC result；JSON 路径不变。

### RPC

新增（capabilities 已登记）：

- `settings/mcp`
- `settings/mcp/upsert`
- `settings/mcp/delete`
- `settings/mcp/probe`

密钥永不回传；`auth_env_set` 只报环境变量是否存在。保存后 `OnSettingsChanged` → `ReplaceServers`。

### UI

- 新视图 `ui/src/components/mcp/McpView.tsx`，路由去掉 `DemoBanner`。
- 表单只收 HTTP 地址 + 可选 `auth_env`。不再提供 STDIO（本迭代伪操作）。
- 导入接受常见 MCP JSON；stdio 项跳过并说明，不假装已接入。
- 删除 `McpDemoView`、`demo-api` MCP CRUD、`DemoMcpServer`。

## 明确不做

- Notebook / Persona / Cron / Skills / Memory / Evolution 去演示化
- 聊天编辑 / 回退 / 分叉、附件 / AutoDream / 语音
- MCP elicitation、stdio 传输、把远端工具升格为独立 catalog 条目
- 引入 `eino-ext` MCP 包

## 变更文件

后端：`internal/app/settings`、`internal/runtime/mcp_backend.go`、`internal/app/app.go`、`internal/rpc/control.go`、`config.example.yaml` 及对应测试。

前端：`ui/src/components/mcp/`、`ui/src/routes/_layout.mcp.tsx`、`ui/src/lib/api.ts`、`ui/src/lib/demo-api.ts`、i18n、e2e。
