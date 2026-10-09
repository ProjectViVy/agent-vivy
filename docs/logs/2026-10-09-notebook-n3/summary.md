# N3 — Real notebook UI (durable sections/documents editor)

Story: `docs/superpowers/plans/notebook-reports/N3.md` on the `notebook` branch.

## Outcome

The demo notebook page (`demo-api` + `vivy.demo.*` localStorage) is gone.
`plugins/vivy-notebook/ui/vivy-notebook/` now hosts a real editor built on the
N2 sealed action contract:

- `src/types.ts` — wire mirrors (16 `vivy.notebook.*` action ids, outcome
  envelope, limits: 256 KiB body / 16 KiB comment / 100-row pages).
- `src/api.ts` — `NotebookClient` over `createModuleActionClient(rpc)`
  (`module.action.invoke`, `{module_id, action_id, input}`, 1 MiB bound).
  Mutations send `{operation_key, request}`; replies decode the
  `{status, data?, error?{code,message,retryable,current_version?,
  current_revision_id?}}` envelope into `NotebookError`. RPC-level capability
  misses (-32004 / -32601 / "not configured|not found|unavailable") map to
  `capability_unavailable` so an omitted generation renders its explicit state.
- `src/editor.tsx` — explicit Save only (no autosave), plain-Markdown editing +
  `react-markdown`/`remark-gfm` preview (no raw HTML), UTF-8 byte counter,
  per-editor epoch guard against stale loads. Conflict (`revision_conflict`)
  keeps the local buffer and offers keep-editing(rebase) / view-current /
  reload; lost-ack retry reuses the same operation key only for an identical
  request (changed request ⇒ new key ⇒ one revision per key). Save states
  pending/saved/conflict/unknown are distinct.
- `src/revisions.tsx` — sequence list with origin badges, head/viewing marks,
  per-item busy, view-into-readonly + adopt through `revisions.adopt`.
- `src/comments.tsx` — active/resolved/deleted tabs, create (optional
  `anchor_revision_id`), inline edit, resolve/delete/restore transitions,
  16 KiB counter, per-item busy.
- `src/view.tsx` — MasterDetail over host `usePluginHost()`; section
  select/create/rename (system-role sections render localized titles),
  entry list/create/move/export/delete/restore, show-deleted toggle,
  dirty-buffer navigation confirm, `capability_unavailable` →
  `notebook-unavailable` view. Editor remounts on `id:version` change so a
  restore/move never leaves stale state on screen.
- `i18n/catalog.json` — full EN+ZH catalog for the new surface.
- `index.tsx` — `demo: true` flag removed; the route is a real module page.
- `internal/modules/notebook/actions.go` — N2 schema fix: comment status enum
  is `["active","resolved","deleted"]` (storage writes `active`, never `open`).
- `ui/vitest.config.ts` — plugin source tests included (staged copies
  excluded); `react-markdown`/`remark-gfm` aliased to the ui install for
  plugin-dir resolution.

## Verification evidence

- `pnpm --dir ui test` → 613/613 (15 notebook tests: api 6, editor 4, view 5).
- `pnpm typecheck` clean; `vite build` clean; i18n completeness PASS.
- `just ci` end-to-end green (see verification.md).
- New Playwright split-pair suite: `ui/playwright.notebook.config.ts` +
  `ui/e2e/notebook.spec.ts` + `ui/e2e/notebook-backend.ts`, against **packed**
  binaries (`recipes/default.vivy.yml`, `recipes/no-notebook.vivy.yml`) and
  Vite :3015 → backend :8797 — 5/5 green.
  Screenshots: `n3-editor-saved.png`, `n3-conflict.png`, `n3-unavailable.png`.
- `vivy-module.yaml` source pin re-pinned (`27daa5c3…` → see file).

## Boundaries kept

- No shell-store import, no raw RPC, no new editor dependency, no
  localStorage content authority; wire carries no client-supplied
  scope/actor/origin (asserted by `api.test.ts`).
- Report actions/buttons intentionally absent (R1/R2 own them); revision
  origin labels already render `generated` for future candidates.
- Seeded system-role sections are readable through explicit reads but are not
  part of chat surfaces.
