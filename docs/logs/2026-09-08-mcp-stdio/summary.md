# MCP stdio slice 1

日期：2026-09-08

## 交付

- 配置与 settings 镜像支持 HTTP/stdio 二选一，stdio 使用 `command`、逐项 `args`、`env_from`（CHILD→HOST）和相对 `cwd`。
- 配置即授权：命令接受 PATH 名或绝对路径，危险 basename denylist；没有新增 execute allowlist。cwd 运行时钳制在 `runtime.workspace_root`，环境只注入最小系统变量与显式引用。
- runtime 通过上游 `transport.NewStdioWithOptions` + `client.NewClient` 构造，首次操作才启动；保留 EinoExt `GetTools`、既有 mcp_list_tools/mcp_call、资源/提示词和治理路径。
- stdio 缺失环境、握手/进程死亡均 fail-closed；下一次操作识别死亡并复用既有 `error` 状态，stdio 不自动重启。Windows `.cmd/.bat` 走 `ComSpec` 兜底。
- app overlay、control RPC、MCP 浏览器设置面、JSON import/export、英文/中文 i18n 已贯通。TUI/sidebar 沿用现有 sidebar route 与 configured/initialized/error 三态，并加法传递 `transport`/`env_missing`，没有新增独立 UI/surface 协议。

## 明确未做

- mcp-go stdio reader 的 raw-frame 字节上界尚未补齐；当前仅保证 decoded/projected bounds。
- Windows 子进程树（Job Object/process group）治理尚未补齐；当前关闭/等待覆盖 mcp-go 管理的当前 child。
- OAuth/TokenStore/needs-auth 仍是 MCP-TRANSPORT-1 slice 2。

相关决策和上游证据：`docs/plans/2026-09-07-mcp-stdio-upstream.md`。
