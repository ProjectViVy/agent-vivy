# Acceptance

Open `http://127.0.0.1:3015/mcp` on the split pair (`just dev`).

1. 页面无「演示 / 本地模拟」横幅，空态为「尚未配置 MCP 服务」。
2. 添加服务：名称 `docs`，地址 `https://example.com/mcp`（或本机真实 Streamable HTTP MCP），可选填写 `auth_env` 变量名。保存后列表出现该项，开关为开。
3. 刷新页面后该项仍在。`localStorage` 无 `vivy.demo.mcp`。
4. 停用开关后，该服务不再进入 `mcp_list_tools` / `mcp_call` 目录。
5. 「探测工具」对已启用服务发起 `tools/list`：成功显示工具数；失败保留配置并显示错误，不从列表消失。
6. 删除需确认，删除后空态回来。
7. 导入含 `command` 的 stdio JSON 时跳过该项并说明，不假装已接入。

Chat path (optional if a real MCP server is running):

8. 启用一台可达的 HTTP MCP 后，新对话里模型可调用 `mcp_list_tools`；`mcp_call` 仍走审批。
