# Session Workspace Picker

## Outcome

PR #30 now groups chats by the real directory attached to each session instead
of browser-only folder labels. The chat input shows the active workspace beside
the context percentage, opens an in-app directory picker, and displays the
selected folder after the user confirms it.

## Architecture

- `sessions.workspace_path` is durable in SQLite and PostgreSQL. An empty value
  means Vivy's existing default private per-run workspace.
- `session/create` accepts a workspace path and `session/set_workspace` changes
  it only before the session's first durable run.
- `workspace/browse` returns bounded, sorted directories from the host without
  returning files or symlink entries.
- The runtime resolves file, command, download, and workspace-preview access
  against the selected session directory. Sessions without a selection retain
  the previous isolated-run behavior; `vivy-code` retains its local project
  workspace behavior.
- Project instructions follow the same authority boundary: a selected session
  receives that folder's `AGENTS.md` and conventional project-skill overlays,
  while a default session retains the launch project's instructions.
- The Web store owns workspace changes. The sidebar derives groups directly
  from backend session data and no longer writes `vivy.demo.sessionFolders*`
  localStorage records.

## UX behavior

- Empty workspace path: **Default workspace**.
- Clicking the input-bar entry opens a folder browser with path entry, roots,
  parent navigation, and subdirectories.
- Selecting a folder updates an unused active chat. If the active chat already
  contains history or has a running turn, Vivy creates and selects a new chat in
  that folder so historical runs never silently change filesystem authority.
- Confirming the already-active workspace is a no-op. Typed paths must be
  browsed successfully before confirmation, and stale directory responses are
  ignored so the displayed and selected paths cannot diverge.
- Sidebar workspace headings use the folder name, retain the exact path in the
  title, and provide a new-session action scoped to that path.
