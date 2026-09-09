# Acceptance

Open `http://127.0.0.1:3015/mcp` on the split pair (`just dev`).

1. The page has no "Demo / local simulation" banner; the empty state is "No MCP
   servers configured".
2. Add a server: name `docs`, address `https://example.com/mcp` (or a real local
   Streamable HTTP MCP), optionally fill in the `auth_env` variable name. After saving,
   the item appears in the list with its toggle on.
3. Refresh the page; the item remains. `localStorage` contains no `vivy.demo.mcp`.
4. After disabling the toggle, the server no longer enters the `mcp_list_tools` /
   `mcp_call` catalog.
5. "Probe tools" sends `tools/list` to an enabled server: success displays the tool
   count; failure retains the config and displays the error without removing the item.
6. Deletion requires confirmation; after deletion the empty state returns.
7. When importing stdio JSON containing `command`, skip that item and explain why;
   do not pretend it was connected.

Chat path (optional if a real MCP server is running):

8. After enabling a reachable HTTP MCP, the model can call `mcp_list_tools` in a new
   conversation; `mcp_call` still goes through approval.
