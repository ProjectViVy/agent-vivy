# Sidebar toolbox drill-down + session folders

## What changed

Reworked the Vivy chat sidebar navigation hierarchy.

### Information architecture

Top-level function entries are now only two drill-downs (the standalone
**聊天** entry was removed on request — it duplicated the session list):

1. **工具箱** (drill-down) — 定时任务, MCP, Skill, 中控台, 审批中心
2. **VIVY** (drill-down) — 人格, 面具, 进化, 记忆, 记事本

Clicking **工具箱** or **VIVY** replaces the sidebar body with that group's
submenu. A **返回** control at the top returns to the root menu. Navigating
into a tool/persona route also auto-selects the matching drill-down view.

### Sessions

Sessions live in the sidebar under a **会话** section (replacing the header
sessions drawer trigger):

- **新建会话** action at the top of the root menu and beside the section label
- Folder grouping: sessions can be moved into named folders; folders expand/
  collapse; ungrouped sessions stay as a flat list
- Per-session rename, delete, and move-to-folder actions
- Folder map and open state persist under `vivy.demo.sessionFolders*`
  (local demo preference only; no backend field)

### Layout / code

- `ui/src/components/chat/ConversationSidebar.tsx` — full rewrite
- `ui/src/components/chat/ChatInput.tsx` — removed the legacy **新建会话**
  Plus button next to send (sidebar session list is the single entry)
- `ui/src/routes/_layout.tsx` — passes session store actions into the sidebar;
  removes the header sessions Sheet trigger; keeps SessionDrawer for the
  ChatInput history entry
- `ui/src/i18n/zh.ts` / `en.ts` — `nav.toolbox`, `nav.vivy`, `nav.back`, and
  `sidebar.*` keys; dropped unused `nav.chat` and `chatInput.newSession*`

## Scope explicitly not done

- No backend `folder` field on `Session`; grouping is UI-local demo state
- No drag-and-drop reordering
- `just ci` full gate not run in this environment (UI lane only: typecheck,
  vitest, vite build). Backend untouched.

## Skill

Guided by `.agents/skills/oil-frontend` (component contract + information and
action contract): one authority for nav structure, no duplicate session entry
in the header, no page-local style patches on shared tokens.
