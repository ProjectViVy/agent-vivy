# Session list header actions (Vivy UI sidebar)

## What changed

The `Sessions` section header in the browser sidebar had a single `+` whose
only behaviour was "create a session in the default workspace". It is now a
harness-style action row with three actions, and folder entry replaces blind
session creation:

- **Search** – expands an input that takes over the section header and filters
  the visible rows by metadata (session title, folder label, or the untitled
  placeholder). A query with no match reports "no matching sessions" instead of
  showing an empty list; `Esc` or the cancel button clears and collapses it.
- **View options** – a menu with `Group by folder` (default) and `Flat list`.
  The choice is browser-local (`vivy.ui.sessionListView`) and survives reload.
- **Choose a folder and enter** – opens the folder picker; the picked folder is
  adopted as the working folder and the app enters the chat for it.

The folder picker itself is the existing `WorkspaceSelector` dialog, lifted into
a shared `WorkspaceFolderDialog` so both surfaces browse and confirm folders
through one implementation. No second folder browser was written.

Entering a folder goes through the store's existing `chooseWorkspace` seam
(`ui/src/routes/_layout.tsx` → `onChooseWorkspace`): an empty draft session
adopts the folder instead of spawning a second session, and only a session with
content causes a new session in that folder. This is why the header action is
not "new session" — the folder is the decision, and the session follows it.

Search is intentionally metadata-only. The control plane exposes no session
content search RPC, so the input never claims to search transcript text.

## Scope

Changed:

- `ui/src/components/chat/ConversationSidebar.tsx` — header action row, search
  and view-mode list projection, folder-entry dialog, `onChooseWorkspace` prop.
- `ui/src/components/chat/WorkspaceFolderDialog.tsx` — new shared picker dialog
  (lifted from `WorkspaceSelector`, same DOM, copy and browse semantics).
- `ui/src/components/chat/WorkspaceSelector.tsx` — now a trigger plus the shared
  dialog; its own behaviour is unchanged.
- `ui/src/components/chat/session-list-view.ts` — pure view-mode persistence and
  metadata filter.
- `ui/src/routes/_layout.tsx` — `chooseFolderAndOpen` wiring.
- `ui/src/i18n/{zh,en}.ts` — `sidebar.search|view|viewGrouped|viewFlat|openFolder`
  and `workspace.enterTitle|enterDescription`.
- Tests: `session-list-view.test.ts`, `ConversationSidebar.test.tsx`.

Explicitly not done:

- No manual drag sorting, and no host-side persistence of the view mode.
- No session-content search (no RPC exists).
- The per-folder hover `+` and the body's `New session` button still create a
  session in that folder / the default workspace respectively.
- No `ui-sdk` Face contract change: the new path reuses `chooseWorkspace`, which
  the Face already exposes. `sdk/ui` and the plugin catalogs are untouched.
- The header keeps its layout position; no sidebar redesign.

## Concurrency note

A second write lane was active in the shared root tree during this change
(i18n `memory` / `notebook` / `persona` / `evolution` namespaces being moved to
the plugin catalogs, plus new `plugins/vivy-*` trees). Every edit here is
additive and confined to the `sidebar` and `workspace` key groups; no line from
that lane was reverted. Its in-flight state is why the repository-wide i18n and
UI test gates are currently red — see `verification.md`.
