# Acceptance

A human can tell this worked when, at `http://127.0.0.1:3015`:

1. The left sidebar shows two drill-down entries under the logo:
   **工具箱**, **VIVY**, plus a **会话** list and **设置**. There is no
   separate **聊天** nav item.
2. Clicking **工具箱** turns the sidebar into a submenu with **返回** and
   定时任务 / MCP / Skill / 中控台 / 审批中心. Clicking **返回** restores
   the root menu.
3. Clicking **VIVY** does the same for 人格 / 面具 / 进化 / 记忆 / 记事本.
4. Opening MCP (or any tool/persona page) from the submenu keeps that group
   as the sidebar view.
5. Sessions appear in the sidebar; **新建会话** creates a session. Hovering a
   session reveals rename / delete / folder. Folders expand and collapse, and
   grouping survives a page refresh (local `vivy.demo.sessionFolders*`).
6. The chat input no longer shows a Plus **新建会话** next to send.
