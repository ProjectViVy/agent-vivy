# VCP E2 verification

Date: 2026-10-06 (UTC). Branch `feat/vivy-code-parity`.

## Focused suite

`go test ./internal/toolhost ./internal/tools ./internal/runtime ./internal/mcphost -run 'Exposure|Activate'` — plus full-package runs:

```
ok  agent-vivy/internal/tools      0.445s
ok  agent-vivy/internal/app        7.902s
ok  agent-vivy/internal/runtime   45.925s
ok  agent-vivy/internal/rpc       16.721s
ok  agent-vivy/internal/toolhost   0.009s
ok  agent-vivy/internal/mcphost    0.012s
ok  agent-vivy/internal/config     0.014s
ok  agent-vivy/internal/domain     0.005s
ok  agent-vivy/internal/app/settings 0.578s
```

New tests:

- `internal/tools/toolactivation_test.go` — tracker set ops, ctx binding, resolve defaults (fixed core → direct, others → deferred, explicit stamp wins).
- `internal/runtime/tool_activation_test.go` — suppressed middleware strips hidden ToolInfos; activated middleware rehydrates activated deferred infos (and drops on deactivate); journal fold survives tracker drop for both `tools.exposure_changed` and `tool_search` finished events, incl. deactivate ordering.
- `internal/app/assembly_exposure_test.go` — resolver precedence (exact > per-server glob most-specific-first > deferred list), invalid levels skipped, hidden rejected on the model path (`tool_not_active`) yet callable internally, deferred callable on the model path (disclosure-only).
- `internal/rpc/control_tools_activation_test.go` — tools/list exposure + activated projection, tools/activate outcome map (activated / hidden / unknown_tool), tools/deactivate.

## Acceptance mapping

| Plan acceptance | Evidence |
| --- | --- |
| Deferred tool invisible to the model | `dynamicTools` hidden by `einotoolsearch` (existing) + suppressed middleware; `tools/list` reports `exposure: deferred` |
| Visible after tools/activate | `activatedToolVisibilityMiddleware` rehydrates deferred infos each generation; rpc test asserts `activated: true` |
| Journal records the change | `tools.exposure_changed` on `sessctl_toolx_<session>`; fold test re-derives state after tracker drop |
| Config-level exposure applies at startup | `resolveToolExposure` stamps specs inside `bindGeneratedTools` during `rebuildToolRegistry` |
| MCP annotation mapping | per-server `tool_exposure` globs matched on `entry.Provenance.ServerInstanceID`, most-specific-first |

## Conformance

Digest re-pinned to `a4a8a24b2080d4b0e6b7bc0bec6921d91d17aadf2ecbf5fc6cc92274e3cbda67` in `sdk/internal/assembly/conformance_results.json`; `go test ./sdk/internal/conformance/` ok (73s).

## Notes

- Mid-story correction: enforcement initially rejected unactivated deferred calls on the model path; that broke the disclosure-only contract and two existing governance e2e tests (`TestProductionMCPGovernancePathUsesToolHostOrder`, `TestPlanGoalIntegratedPendingReviewRecovery`). Only `hidden` is an execution wall now; both e2e tests pass again.
- MCP glob rule order is most-specific-first (longer pattern wins, lexicographic tie-break) — `mcp.docs.secret_*` beats `mcp.docs.*`.
