# Session Workspace Picker Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace PR #30's UI-local session folders with real, durable workspace selection and automatic chat grouping by the directory Vivy actually uses.

**Architecture:** `Session.WorkspacePath` is the backend-owned association. An empty value means Vivy's existing default per-run workspace; a non-empty value is a canonical existing directory mounted with the same local-world rules as `vivy-code`. The browser uses a bounded directory-browsing RPC rather than inventing a path in `localStorage`, and the runtime resolves every run through the session association.

**Tech Stack:** Go 1.24, SQLite/Postgres storage, JSON-RPC, React 19, TypeScript, Zustand, Vitest, Playwright.

**Spec:** The acceptance contract in PR #30 plus the owner's 2026-09-14 correction: selector to the right of context usage, default workspace fallback, folder dialog, selected workspace display, and sidebar grouping by selected directory.

## Global Constraints

- Keep one `Service.Run`, Journal, policy, sandbox, and filesystem path.
- Do not persist real workspace state in `vivy.demo.*` or any browser storage.
- Preserve `vivy-code` local-world behavior and the ordinary web default workspace behavior.
- A session workspace is mutable only before its first run, so historical run-to-workspace identity cannot drift.
- Directory browsing is local, directory-only, symlink-safe, sorted, and bounded.
- All user-facing strings are localized in English and Simplified Chinese; documentation remains English.

---

### Task 1: Durable session workspace contract

**Files:**
- Modify: `internal/domain/session.go`
- Modify: `internal/storage/contracts.go`
- Modify: `internal/storage/sqlite/sqlite.go`
- Modify: `internal/storage/sqlite/sessions.go`
- Modify: `internal/storage/postgres/schema.go`
- Modify: `internal/storage/postgres/postgres.go`
- Modify: `internal/storage/postgres/sessions.go`
- Modify: `internal/storage/conformance/suite.go`
- Modify: `internal/rpc/control.go`
- Test: `internal/storage/conformance/suite.go`
- Test: `internal/rpc/control_test.go`

**Interfaces:**
- Produces: `domain.Session.WorkspacePath string`; `storage.SessionWorkspaceStore.UpdateSessionWorkspace(context.Context, domain.SessionID, string) error`; `session/create.workspace_path`; `session/set_workspace`; `sessionResult.workspace_path`.

- [ ] Write a storage conformance case proving create/list/get round-trip and first-run immutability.
- [ ] Run the SQLite conformance test and confirm the missing field/method fails.
- [ ] Add schema migrations and backend reads/writes with an atomic `NOT EXISTS` run guard.
- [ ] Run SQLite and Postgres storage tests until the contract passes.
- [ ] Write RPC tests for canonical selection, default reset, invalid directories, and started-session conflict.
- [ ] Implement the minimal RPC projection and mutation path, then rerun the focused RPC tests.

### Task 2: Session-aware runtime workspace resolution

**Files:**
- Modify: `internal/runtime/isolation.go`
- Modify: `internal/runtime/isolation_test.go`
- Modify: `internal/runtime/service.go`
- Modify: `internal/runtime/sandbox_manager.go`
- Modify: `internal/runtime/sandbox_manager_test.go`
- Modify: `internal/runtime/filesystem_backend.go`
- Modify: `internal/runtime/command_backend.go`
- Modify: `internal/runtime/download.go`
- Modify: `internal/runtime/workspace_files.go`
- Modify: `internal/app/app.go`
- Modify: `internal/modules/sandbox/module.go`

**Interfaces:**
- Consumes: session workspace and run ownership from Task 1.
- Produces: `NewSessionWorkspaceManager(defaultRoot, sessions, runs)` and root-aware sandbox validation.

- [ ] Write a failing runtime test with one default session and two selected-directory sessions.
- [ ] Confirm it fails because `WorkspaceManager` always uses the process root.
- [ ] Resolve a run's session from context before persistence and from `RunStore` afterward; return the default private run directory or canonical selected directory.
- [ ] Make filesystem, command, download, preview, and LSP paths validate against the resolved workspace root.
- [ ] Run focused runtime/app tests and confirm default, local code-face, restart, child-run, and escape behavior remains green.

### Task 3: Bounded directory browser RPC

**Files:**
- Create: `internal/rpc/workspace_picker.go`
- Create: `internal/rpc/workspace_picker_test.go`
- Modify: `internal/rpc/control.go`
- Modify: `ui/src/lib/api.ts`

**Interfaces:**
- Produces: `workspace/browse { path? } -> { path, parent?, roots, directories }`, where every returned directory has `name` and canonical `path`.

- [ ] Write failing tests for home fallback, parent/root navigation, stable sorted output, symlink omission, and invalid path rejection.
- [ ] Run the focused RPC test and verify the missing method failure.
- [ ] Implement the bounded browser and capability advertisement without shelling out or adding dependencies.
- [ ] Add typed TypeScript request/response helpers and rerun Go RPC plus TypeScript API tests.

### Task 4: Workspace selector and automatic sidebar grouping

**Files:**
- Create: `ui/src/components/chat/session-workspaces.ts`
- Create: `ui/src/components/chat/session-workspaces.test.ts`
- Create: `ui/src/components/chat/WorkspaceSelector.tsx`
- Create: `ui/src/components/chat/WorkspaceSelector.test.tsx`
- Modify: `ui/src/components/chat/ChatInput.tsx`
- Modify: `ui/src/components/chat/ConversationSidebar.tsx`
- Modify: `ui/src/lib/store.ts`
- Modify: `ui/src/lib/store.test.ts`
- Modify: `ui/src/routes/_layout.tsx`
- Modify: `ui/src/i18n/en.ts`
- Modify: `ui/src/i18n/zh.ts`

**Interfaces:**
- Consumes: `Session.workspace_path`, `api.browseWorkspace`, `api.setSessionWorkspace`.
- Produces: input-bar selector immediately after context usage; authoritative workspace groups in the session sidebar.

- [ ] Write failing pure grouping tests proving default and canonical paths produce deterministic groups.
- [ ] Write failing store tests proving an empty session is updated while a started session creates and selects a new workspace-bound session.
- [ ] Implement the store actions and rerun the focused tests.
- [ ] Write the selector render/interaction test for default label, dialog navigation, selected label, and error state.
- [ ] Implement the dialog and place its trigger to the right of context usage.
- [ ] Replace local folder maps, rename/pin/drag state, and `vivy.demo.sessionFolders*` with workspace-derived collapsible groups.
- [ ] Run `pnpm typecheck`, focused Vitest files, and the full UI suite.

### Task 5: Product evidence and PR update

**Files:**
- Modify: `docs/logs/2026-09-13-sidebar-toolbox-drilldown/summary.md`
- Modify: `docs/logs/2026-09-13-sidebar-toolbox-drilldown/verification.md`
- Modify: `docs/logs/2026-09-13-sidebar-toolbox-drilldown/acceptance.md`

**Interfaces:**
- Consumes: all prior tasks.
- Produces: merge-ready evidence on PR #30.

- [ ] Run focused Go tests, full UI tests, typecheck, build, and `just ci`.
- [ ] Start the split development pair and smoke default selection, directory navigation, workspace switching, refresh persistence, sidebar grouping, and one real run workspace.
- [ ] Update the existing iteration evidence with exact commands/results and remove claims that grouping is UI-local.
- [ ] Inspect `git diff --check`, status, and the final diff; commit one focused correction.
- [ ] Push the commit to `feat/sidebar-toolbox-drilldown`, then re-fetch PR #30 and report its current checks and mergeability.
