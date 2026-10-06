# H1 — Coding bundle consolidation

**Goal:** non-protected coding capability physically organized under `plugins/coding/*`; `recipes/vivy-code.vivy.yml` is the bundle definition; Inspect reports the whole inventory.
**Epic:** H. **Requirements:** RQ-BND. **Predecessors:** E2, E3, C1, A2, A3 (the things being consolidated must exist).
**Spec:** VCP-D1 §2 + §5.9. **Refactor-class story — every moved ID must keep its runtime identity.**

## Scope

**Files:** move non-protected coding tools from `internal/tools` → `plugins/coding/<name>/` as public `std/tool@v1` modules: `agent`, `workflow`, `notes` (write/list/read), `network_search`, `http_request`, `web_fetch`, `download`, `sequential_thinking`, `enter_plan_mode`/`submit_plan`, `get_goal`/`create_goal`/`report_goal`, `job_output`/`job_kill`, `child_inbox`, `reply_parent`. Protected IDs (the 11 in PORT-CATALOG §3) NEVER move. Recipe: add `coding/*` modules to `vivy-code.vivy.yml`; descriptor digests + `conformance_results.json` re-pin.

## Tasks

- [ ] Inventory final tool-ID ownership map (what stays T1 vs moves) — get owner sign-off on the table before moving code.
- [ ] Per moved tool: new module dir, `std/tool@v1` Provider wrapping existing implementation (minimal adapter — do not re-architect internals), descriptor + source digest.
- [ ] Grouping decision: one `coding-tools` module vs per-domain modules (`coding-web`, `coding-tasks`, `coding-agents`, `coding-jobs`) — prefer per-domain for selective recipes; record choice.
- [ ] Recipe update + pack + Inspect inventory evidence proving all IDs present.
- [ ] Regression: default Generation unchanged; `vivy-code` Generation gains the bundle; tests that referenced internal tool paths updated.
- [ ] `just ci` full green incl. `TestPackAndInspectEveryShippedRecipe`.
- [ ] Commit `feat(coding): consolidate coding capability into plugins/coding bundle`.

## Boundary

Behavior-preserving move only — no feature changes inside this story. Tool IDs, schemas, Journal event names all frozen.

## Acceptance

A packed `vivy-code` generation's Inspect shows the bundle inventory; a diffed `tools/list` vs pre-move main is empty.
