# VC-3g: UI file preview + syntax highlighting (workspace files panel)

Date: 2026-09-01 | Branch: `feat/vc1a-bash-tool` | Worktree: `agent-vivy-vc0`

## What changed

VC-3 slice 7: add a "current run workspace files" preview panel (Files) to the
UI, with syntax highlighting.

### Kernel (read-only WorkspaceFiles service + RPC)

- `internal/runtime/workspace_files.go` (new):
  - `WorkspaceFiles`: read-only accessor for a run workspace, used by the control
    plane UI for preview.
  - `List(ctx, runID)`: walks the workspace root (`WorkspaceManager.Ensure` lazily
    creates the directory), returns relative paths (forward slashes) + sizes,
    sorted by path; skips symlinks (and entire symlinked directories); caps each
    response at 2000 items, reports `truncated` when exceeded, and never streams
    unbounded data to the UI.
  - `Read(ctx, runID, path)`: reads one file. Path safety is double-enforced:
    `workspaceRelPath` sanitizes (rejects empty values/backslashes/drive-letter
    colons/leading slashes, and rejects `.`/`..`/`../` prefixes after
    `path.Clean`) + `WorkspaceManager.ValidatePath` rechecks the combined
    absolute path; `Lstat` rejects symlinks and non-regular files; binary files
    (`isBinary`) return only a flag, not content; text beyond the byte limit is
    truncated with the same default as the filesystem tools and marked
    `truncated`.
- `internal/rpc/control.go`: adds `workspace/list` and `workspace/read`; the
  `ControlDeps.WorkspaceFiles` interface is dependency-injected, with nil →
  `MethodNotFound` "workspace files are not configured" following the MCPCatalog
  precedent; missing `run_id`/`path` → `InvalidParams`. All internal errors
  collapse to `-32603 internal error` without exposing path-validation details to
  clients.
- `internal/app/app.go`: assembles `WorkspaceFiles` (keeps it nil when
  workspaceManager is nil, thereby disabling the methods).

### UI (Files side panel)

- `ui/src/components/files/FilesPanel.tsx` (new): no run → empty state; with a
   run → list (path/size/count/truncation marker/refresh) + click-to-preview;
   syntax highlighting uses `highlight.js` (`lib/common` on-demand subset +
   `github-dark.css`; extensions map through `hljs.getLanguage`, unknown
   extensions fall back to HTML-escaped plain text). hljs output is already
   HTML-escaped markup; `escapeHtml` is used only on the fallback path, so
   `dangerouslySetInnerHTML` is safe here.
- `ui/src/lib/api.ts`: `WorkspaceFile`/`WorkspaceFileContent` types +
  `listWorkspaceFiles`/`readWorkspaceFile` (POST body in snake_case, matching the
  control-plane contract).
- `ui/src/lib/store.ts`: `filesPanelOpen` state (same Sheet toggle as Review
  Center).
- `ui/src/routes/_layout.tsx`: add a Files icon button (Folder, aria-expanded) to
  the header and a right-side Sheet (`sm:max-w-[560px]`), alongside Review Center
  / Todos.
- `ui/src/i18n/{en,zh}.ts`: `layout.files` + `files.*` copy group (empty state /
  count / truncation / binary / loading / selection hint).
- `ui/package.json` + `pnpm-lock.yaml`: the only new UI dependency,
  `highlight.js`.

### e2e and harness

- `ui/e2e/files-panel.spec.ts` (new): shell-state case that runs without a
  provider—the welcome wizard is skipped → open Files → assert the empty-state
  copy → Escape closes → reopening remains empty.
- `ui/e2e/global-setup.ts`: e2e config explicitly adds `runtime.workspace_root`
  (pointing to `.e2e-workdir/workspace`). This fixes a smoke-test gap: without
  workspace_root, any path that triggered `Ensure` previously fell back to the
  default user root `~/.vivy/workspace`.

## Design decisions

- Read-only surface: the Files panel only consumes `workspace/list` /
  `workspace/read`; it provides no write/delete/download capability—preview is
  the whole surface, and restore semantics belong to RB-1 file-version history
  (awaiting O1..O6).
- Disabled means dependency missing: as with MCPCatalog, an unassembled
  `WorkspaceFiles` returns MethodNotFound rather than an empty result (honestly
  reflecting missing capability).
- Highlighter choice: highlight.js is the only new UI dependency; language
  mapping conservatively passes an extension only when it is recognized, otherwise
  it escapes plain text without guessing the language.
- Binary content is not returned: UI preview is a text surface; image preview
  uses the read_file tool chain (slice 5) and is not duplicated here.

## Not done (explicitly not done)

- File-version history (the last VC-3 item): awaiting the O1..O6 user ruling
  (RB-1).
- Directory tree/recursive browsing/search: Crush has no such surface, so it was
  not added.
- Image/binary content preview: likewise, the panel only provides text preview.
- End-to-end browsing inside a run with a provider: this machine has no provider
  key and cannot run a real run; the kernel path was covered by a real-server
  WebSocket smoke test (see verification.md).

## Acceptance basis

- Crush is FSL-1.1-MIT: this slice is Vivy-owned capability (workspace file
  preview panel), aligning with Crush's "file visibility" goal with zero code
  copying or upstream excerpts.
- Browser verification on a machine without a provider is limited to shell state;
  the full RPC path is recorded in the real-server smoke test.
