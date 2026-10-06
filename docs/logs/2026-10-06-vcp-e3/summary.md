# VCP E3 — tool_search over the deferred catalog

Story: `docs/superpowers/plans/vivy-code-parity/E3-tool-search.md`
Commit message: `feat(runtime): tool_search deferred-catalog e2e`

## Deviation from plan (recorded)

The plan drafted a `plugins/coding/tools` module implementing a
BM25-ish `tool_search` tool. Discovery during E2 showed the pinned
**Eino `adk/middlewares/dynamictool/toolsearch`** already provides the
complete model-side surface: a `tool_search` meta-tool
(`{query, max_results}`, default 5), keyword/`select:` matching over the
dynamic (deferred) catalog, forward-selection of matched ToolInfos on
the next model leg, and a system reminder listing available names.
Building a second implementation would duplicate the pinned Eino
surface — disallowed by architecture decision order #2 (Eino-native
capability second; custom code is the exception).

Vivy's E2 layer supplies the parts pi needs beyond Eino's per-run
forward selection:

- `tool.finished(tool_search)` folds `{"matches":[...]}` into the
  session `ToolActivation` set (`noteToolSearchMatches` in
  `persistAndPublish`), so matches activate session-wide, not just for
  the rest of the current run.
- `activatedToolVisibilityMiddleware` rehydrates activated deferred
  ToolInfos every model generation (the projection survives across runs
  via the Journal fold).
- `hidden`-exposure tools are excluded from `dynamicTools`, so they can
  never appear in search results — matching the plan's "hidden tools
  never appear" requirement by construction.
- Direct tools are always disclosed, so they never need searching or
  activation — plan requirement satisfied structurally.
- Namespace-aware display comes from tool naming (`mcp.<server>.<cap>`)
  inside Eino's system reminder list.

No `plugins/coding/tools` module was created; the remaining plugin-side
catalog surface lands with H1 bundle consolidation if still needed.

## What landed

`internal/runtime/tool_search_e2e_test.go` —
`TestToolSearchActivatesDeferredTool`: scripted model → `tool_search
{"query":"echo"}` → journal `tool.finished` carries `{"matches":
["echo_info"]}` → every subsequent `model.request` lists `echo_info` in
`SelectedTools` → the deferred `echo_info` executes →
`svc.ToolActivation` returns `[echo_info]`.
