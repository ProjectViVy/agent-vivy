# Eino tool-search migration

## Delivered

- Replaced Vivy's custom dynamic search/visibility path with the pinned Eino
  v0.9.13 core middleware
  `github.com/cloudwego/eino/adk/middlewares/dynamictool/toolsearch`.
- Kept the exact fixed-visible core in the static ToolsNode; remaining
  allowlisted active tools are deferred once to the official middleware.
- Kept hidden tools as governed Vivy adapters and added a minimal final
  Skill-mount projection that rehydrates missing persisted `ToolInfos`.
- Kept Vivy allowlist, policy, HITL, hooks, output limits, and second-call
  checks on business tools. The official raw `tool_search` meta-tool is not
  wrapped by the Vivy adapter.
- Removed the retired custom tool-search implementation and registry entry.
- Added one shared config helper used at config/settings boundaries to remove
  legacy `tool_search`, preserve configured order, and turn legacy-only input
  into an explicit empty active surface. Empty `tools.enabled` is valid.
- Updated Eino references, the living TODO board, and this iteration log.

## Explicitly not changed

Journal, Policy, HITL, Skill backend semantics, plugin contracts, and the
domain/Eino import firewall remain in their existing seams. Branch landing
and review mechanics are left to the delivery owner.

## Eino capability check and custom boundary

The pinned source and tests for
`github.com/cloudwego/eino/adk/middlewares/dynamictool/toolsearch` were
inspected. Its `Config{DynamicTools, UseModelToolSearch}` and `New` constructor
provide the required client-side deferred search, duplicate-name validation,
and selection rehydration. The official middleware cannot observe Vivy's
external Skill mount state or restore a hidden `ToolInfo` that was omitted by
the persisted Eino state. A normal business-tool adapter also cannot solve
that model-visible-state gap, so the only custom seam retained is the small
final mount projection. It can be removed when Eino exposes an external
mount/re-hydration hook with equivalent state semantics.

Legacy `tool_search` normalization is intentionally a compatibility bridge at
the config/settings input and write boundaries. It is kept for one release
cycle; the next config schema version should remove it after migration
telemetry/upgrade guidance confirms no old documents remain.
