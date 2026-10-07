# VCP E2 — Tool exposure levels + session activation

Story: `docs/superpowers/plans/vivy-code-parity/E2-tool-exposure.md`
Commit message: `feat(tools): exposure levels with session activation`

## What landed

- **`domain.ToolSpec.Exposure`** with `ToolExposure` ∈ {unset, direct, model-only, deferred, hidden} and `ParseToolExposure`.
- **`tools.ResolveToolExposure(spec)`** — explicit stamp wins; unset falls back to the pre-E2 engine split (fixed core → direct, everything else → deferred), so default behavior is unchanged.
- **`tools.ToolActivation`** — session-scoped, journal-folded activation set (ctx-bound like `MountedTools`).
- **Engine projection**: hidden tools are stripped from model-visible `ToolInfos` every generation (`suppressedToolVisibilityMiddleware`); session-activated deferred tools are rehydrated every generation (`activatedToolVisibilityMiddleware`, same pattern as skill mounts). Eino `einotoolsearch` keeps owning the search surface — E2 formalizes the levels, it does not replace the mechanism.
- **Journal event `tools.exposure_changed`** folded via `sessionToolActivation` (mirrors `sessionMounts`): replays every real run plus the session's synthetic control run `sessctl_toolx_<session>` — real runs seal at their terminal event, so control flips journal on a non-sealing synthetic run (same mechanism as manual compaction events). `tool.finished` for `tool_search` folds `{"matches":[]}` into the set; `noteToolSearchMatches` feeds it live inside `persistAndPublish`.
- **RPC**: `tools/activate` / `tools/deactivate` `{session_id, ids}` → `Service.SetToolActivation` (journal + live tracker). `tools/list` gains optional `session_id` and reports `exposure` + `activated` per entry.
- **Config**: `tools.exposure.<name>`, `tools.deferred_tools: []`, per-server `mcp.tool_exposure` glob→level rules (most-specific pattern first), plumbed through `config.MCPServer` / `settings.MCPServer` / `runtime.MCPServerConfig`.
- **Enforcement** at the ToolHost boundary (`governedTool.InvokableRun`): a model-dispatched call (bound `ToolCallID`) to a `hidden` tool fails `tool_not_active`; internal calls (no marker) are unrestricted. Deferred is disclosure-only — an activated or search-named deferred tool executes normally.

## Design notes

- Deferred = disclosure level, not an execution wall. A model may learn a deferred tool's name through `tool_search` results or activation; rejecting the call itself would break both pi semantics and pre-E2 runs.
- The synthetic control run keeps `tools.exposure_changed` durable when the session's latest real run has already sealed; the fold covers both real and synthetic runs.
- Unknown / mistyped levels are skipped with a warn log — config typos never widen or destroy the tool surface.
