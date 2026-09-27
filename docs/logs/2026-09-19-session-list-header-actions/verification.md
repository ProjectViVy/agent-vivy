# Verification — session list header actions

Date: 2026-09-19. Working tree: shared root checkout (see the concurrency note).

## Commands and results

| Check | Command | Result |
|---|---|---|
| Focused UI tests | `cd ui; npx vitest run src/components/chat/ConversationSidebar.test.tsx src/components/chat/session-list-view.test.ts src/components/chat/WorkspaceSelector.test.tsx` | PASS — 3 files, 23 tests. Includes the 8 pre-existing `WorkspaceSelector` tests, which prove the picker extraction preserved the dialog's DOM, copy and browse semantics. |
| UI typecheck | `cd ui; pnpm typecheck` | PASS (`tsc --noEmit`, no output) |
| UI build | `cd ui; pnpm build` | PASS — 2379 modules, `dist/assets/index-B4QVtBkx.js` 1397.44 kB |
| Full UI suite | `cd ui; pnpm test` | 386 passed, 2 failed — both in `src/i18n/index.test.ts` asserting `t('notebook.reports') === 'Reports'`; see "Other lane" below. No test file touched by this change failed. |
| i18n completeness | `node scripts/check-i18n-completeness.js --root .` | FAIL — 109 errors, every one an `Unknown translation` in the `evolution`, `memory`, `notebook` or `persona` namespaces. Filtering the output for `sidebar.`, `workspace.enter`, `WorkspaceFolderDialog`, `ConversationSidebar` and `session-list-view` returns nothing, i.e. no error belongs to this change. |
| Product gate | `just ci` | FAIL — stops in `ui-core` (line 52) at `pnpm test` with the same two `notebook.reports` failures; `i18n-check: ui-core` means the completeness script never ran inside it. |
| Diff hygiene | `git diff --check` | PASS (no whitespace errors) |

## Real-path smoke at 127.0.0.1:3015 (split Vite, live)

The split pair was already running (`:8787` control plane, `:3015` Vite), so the
change was exercised against the running dev server rather than a rebuilt
embedded UI.

- `GET http://127.0.0.1:3015/` → 200.
- `GET http://127.0.0.1:3015/src/components/chat/ConversationSidebar.tsx` →
  Vite-transformed module contains `sidebar.openFolder`, `sidebar.viewFlat`,
  `WorkspaceFolderDialog` and `onChooseWorkspace`.
- `GET http://127.0.0.1:3015/src/components/chat/WorkspaceFolderDialog.tsx` →
  200, transformed module exporting `WorkspaceFolderDialog`.
- Live control-plane RPC (`/rpc/bootstrap` → WebSocket → `initialize` →
  `workspace/browse`), the exact call the picker makes when it opens from the
  session list with `startPath=''`:

  ```text
  browse("")   -> path C:\Users\Administrator
                  roots C:\, D:\, F:\
                  dirs  [.adal, .agent-browser, .agent-diva]  truncated false
  browse(pick) -> requested C:\Users\Administrator\.adal
                  path      C:\Users\Administrator\.adal
                  dirs      [skills]
  ```

  So the picker opens on a real folder with real roots and can be drilled into;
  the folder action is not an empty dialog.

Limitation: no browser automation was used. The Playwright browsers are not
installed in this checkout (`%LOCALAPPDATA%\ms-playwright` holds no Chromium),
so `just ui-e2e` cannot run here and no screenshot was taken. The header row,
the picker wiring, the search filter, the view-mode switch and their failure
paths are instead covered by the happy-dom tests listed above, and the served
modules plus the live `workspace/browse` call prove the running dev server has
the change.

## Other lane (why the repository-wide gate is red)

A second write lane was active in the same root checkout while this change was
made: `ui/src/i18n/{en,zh}.ts` lost the `memory`, `notebook`, `persona` and
`evolution` namespaces in lockstep (146 lines each), `sdk/ui/src/module.ts` and
`scripts/i18n-cross-face-contract.json` changed, and new `plugins/vivy-*` trees
appeared. Two consequences, both outside this change:

1. `ui/src/i18n/index.test.ts:102,163` still assert `t('notebook.reports')`,
   which that lane removed from the UI catalog.
2. The completeness script still finds 109 UI references to the removed
   namespaces.

This change never edited those namespaces; it only added `sidebar.search`,
`sidebar.view`, `sidebar.viewGrouped`, `sidebar.viewFlat`, `sidebar.openFolder`,
`workspace.enterTitle` and `workspace.enterDescription` to both catalogs. Once
that lane finishes (its UI references and tests reconciled), `just ci` should be
re-run; nothing in this change is expected to fail it.

## Skipped checks

- `just ui-e2e` — Playwright browsers absent (see above).
- `sdk/ui` typecheck — not touched; the new path reuses `chooseWorkspace`, which
  the Face already exposes, so no contract change was needed.
