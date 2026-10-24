# C3 — GUI session tree page + export

**Goal:** session tree visualization + export download in the web face (likely a `coding-ui` ui-extension page or core page — decide at implementation: page contribution via `std/ui-extension@v1` keeps the shell clean).
**Epic:** C. **Requirements:** RQ-SESS, RQ-GUI. **Predecessor:** C1.
**Spec:** VCP-D1 §5.4 + §5.9.

## Scope

**Files:** `plugins/coding/ui/` (or `ui/src` core page — pick one home per ui/AGENTS.md assembled-shell rule; a Module page via `defineUIRoute` is preferred to keep the bundle self-contained), `ui/src/lib/api.ts` (+`session/tree|clone|import|export`), store bindings, sdk/ui Face compat.

## Tasks

- [ ] Tree page: session nodes w/ fork edges, click-to-switch, clone/import/export actions.
- [ ] Export triggers `session/export` then downloads the file through the deliverables/read path (reuse verified-download mechanism; do not inline HTML into the page).
- [ ] Import: file picker → upload → `session/import`.
- [ ] sdk/ui Face mirrors new ops; compat test; i18n.
- [ ] Vitest + browser smoke.
- [ ] Commit `feat(ui): session tree page and export`.

## Boundary

Same trust rules as core pages; export file is served only through the verified transfer path.

## Acceptance

Tree page shows real fork graph; export downloads a byte-verified HTML.
