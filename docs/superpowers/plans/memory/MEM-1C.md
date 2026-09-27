# MEM-1B/1C — Memory UI: full management surface

Story ID: **MEM-1C** | Depends on: MEM-1A (runtime wiring) + MEM-1B (agent tools, merged on same branch) | Req: REQ-MEM-3

## Goal

Turn `plugins/vivy-memory` `/memory` view from a read-only list into the full operator management surface the backend already exposes. The nine `vivy.memory.*` control actions are live; the view today calls only `list`/`search`.

## Global constraints

- Edit `plugins/vivy-memory/ui/vivy-memory/src/**` only (staged copies under `ui/src/generated/` are regenerated — never hand-edit). Extend `memory-client.ts` to the remaining actions; keep the existing `MemoryActionTransport` seam.
- Reuse existing UI kit (`@/components/ui/*`, `MasterDetail`, `Badge`, `Card`, `Input`, `Button`, dialog/confirm if present) and `usePluginTranslation` for all strings — add keys to `plugins/vivy-memory/i18n/catalog.json` in BOTH `en` and `zh` (check the catalog's structure/conventions first).
- Outcome discipline: every invoke returns the four-state envelope — `failed` surfaces `reason` inline (toast or inline alert matching existing patterns); never fabricate success. `applied` triggers a reload so the list is backend-truth.
- No backend changes unless a real contract gap is found (report NEEDS_CONTEXT instead of inventing).
- Keep it minimal per 多快好省: management surface for what the backend serves today (long_term records, machine-memory-home scope) — no speculative features (no history browser, no import/export UI, no Laputa/Garden anything).

## Task 1 — CRUD surface

Extend `memory-client.ts`: `get`, `add`, `update`, `remove`, `rulesRead`, `rulesWrite`, `status` plus input types.

View changes (`view.tsx` + new sibling components as needed):
- **Add**: "New memory" action (header button) → dialog/sheet with content textarea; optional evidence_refs out of scope for v1 (fields exist in DTO — display only). On `applied`, reload + select new record.
- **Edit**: detail pane gets an Edit action → edit mode with content textarea carrying `base_revision` from the record; CAS conflict (`memory_revision_conflict`) shows a non-destructive inline state ("record changed — reload to retry"), NOT silent overwrite. On applied, reload.
- **Delete**: destructive action → confirm dialog that REQUIRES a reason (backend schema requires `reason` + `base_revision`); on applied, reload + clear selection.
- **Detail pane**: render the full DTO — id, trust badge (existing), provenance, sensitivity if present, evidence_refs list (source + uri), created/updated, revision.
- **List**: keep search-debounce; add cursor/`limit` paging only if the outcome actually returns `next_cursor` (check the action's real payload — if absent, single-page with generous limit, do not invent paging).

## Task 2 — MEMRULES editor + status

- A second tab/section (or sidebar-mode switcher, matching existing app patterns — check how other plugin views do tabs) for **MEMRULES**: `rules.read` renders the markdown read-only by default with source badge (file/default); Edit → textarea + Save under digest CAS (`base_revision` = returned `revision` token); conflict → same non-destructive reload-retry state.
- **Status line/card**: `vivy.memory.status` → available, database_present, startup_revision, rules_revision. Small, unobtrusive (footer or status card); when `available=false`, the whole view shows the unavailable state instead of empty lists.

## Task 3 — Gates + log

- `pnpm test`/`vitest` for the plugin view (extend `view.test.tsx` — the existing test mocks the transport seam), `pnpm typecheck`, `just ci` note.
- i18n completeness check passes (`node scripts/check-i18n-completeness.js` runs under `just ci`; en/zh parity required).
- Iteration log `docs/logs/2026-09-27-memory-ui/` (summary/verification/acceptance) + index row → Done.

## Review focus

- Failed outcomes render reason; applied reloads; CAS conflicts never overwrite.
- All user strings i18n'd both locales; no hardcoded copy.
- Readonly view still works when `host` absent / store unavailable.
- No backend edits, no generated-dir edits, no new deps unless already in ui package.json.
