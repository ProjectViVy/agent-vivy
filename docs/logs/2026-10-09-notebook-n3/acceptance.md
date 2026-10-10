# N3 acceptance

Checklist from `docs/superpowers/plans/notebook-reports/N3.md`.

## Task 1 — API/type adaptation layer (RED→GREEN)

- [x] `api.ts` + `types.ts` + `api.test.ts` created; `view.tsx` rewritten;
      `ui/vitest.config.ts` includes plugin sources and excludes staged copies.
- [x] Tests assert action module/id, expected version/base_revision, one
      operation key per request, typed conflict decode, and zero
      client-supplied actor/scope/origin fields.
- [x] Demo page calls into `demo-api` removed — `index.tsx` no longer marks
      the route `demo`, no `vivy.demo.*` keys remain.

## Task 2 — Editor + revisions + comments + export

- [x] `editor.tsx`, `editor.test.tsx`, `comments.tsx`, `revisions.tsx`,
      `view.test.tsx` created; `view.tsx` + `i18n/catalog.json` updated.
- [x] MasterDetail reused; explicit Save; plain-Markdown edit +
      `react-markdown`/`remark-gfm` safe preview; dirty navigation confirm.
- [x] `TestNotebookEditorPreservesDraftOnConflict` green; same-key retry for
      identical requests; pending/saved/conflict/unknown states distinct.
- [x] UTF-8 byte counters (body 256 KiB, comment 16 KiB); export of the
      selected revision emits `.md` + `.sidecar.json` with provenance;
      ActionHost-limit error text surfaced via the error banner.
- [x] EN/ZH catalogs; accessible empty/error states; stale-response epoch
      guards on sections/entries/editor/revisions/comments.
- [x] `pnpm --dir ui test` + `pnpm --dir ui typecheck` green.

## Task 3 — Real split-pair e2e

- [x] `ui/playwright.notebook.config.ts` + `ui/e2e/notebook.spec.ts` +
      `ui/e2e/notebook-backend.ts`.
- [x] Real disposable packed backend + Vite :3015; create section/doc, save,
      comment, restart persistence, two-tab stale save preserving draft,
      move/delete/restore, export download compare — 5/5.
- [x] Seeded notebook marker excluded from chat, readable on explicit read.
- [x] UI without reports: no report-generation controls; packed `no-notebook`
      generation renders the unavailable state.
- [x] Real commands + screenshots recorded (verification.md).
- [x] Module source pin updated through the official stage-ui check;
      `just ci` green.

## Task 4 — Logs, index, commit

- [x] This log directory; index row marked Done; commit
      `feat(notebook): replace demo page with durable editor`.

## Story-level checks

- [x] NOTEBOOK release works without reports/memory: the module's UI and
      actions depend only on `vivy/notebook-core` + host kit; no LAPUTA or
      report-code coupling exists in the plugin.
- [x] Dirty-buffer preservation, same-key retry, cross-scope/section
      navigation guard, accessible empty/error states, real split UI, and
      module omission all covered by tests above.
- [x] Handoff to R2: `NotebookEditor`, `NotebookRevisions`,
      `NotebookComments` take a `NotebookClient` prop (transport-agnostic);
      the e2e helper + packed-backend fixture are reusable for report flows.
