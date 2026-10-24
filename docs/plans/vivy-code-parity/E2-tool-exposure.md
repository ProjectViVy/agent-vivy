# E2 — Tool exposure levels + runtime activation

**Goal:** `domain.ToolSpec.Exposure` ∈ {direct, model-only, deferred, hidden}; model-visible catalog filtering; `tools/activate` control path; Journal event `tools.exposure_changed`.
**Epic:** E. **Requirements:** RQ-TOOL.
**Spec:** [VCP-D1](../../specs/2026-10-06-vivy-code-parity-design.md) §5.6 + note: exposure lives in ToolHost, not middleware (middleware sees calls only at execution). **Baseline:** `f34f3ce`.

## Scope

**Files:** `internal/domain` (ToolSpec field), `internal/toolhost` (catalog filtering + session visibility state), `internal/tools/tools.go` (set exposure on registrations — most stay `direct`), `internal/rpc` (`tools/activate`, `tools/list` gains exposure field), `internal/config` (`tools.exposure` map, `deferred_tools` list), `internal/mcphost` (map MCP tool annotations → exposure hints; MCP `exposure` config per server).

## Tasks

- [ ] `Exposure` on `domain.ToolSpec`; default `direct`. `hidden` = never model-visible (internal callers only); `model-only` = visible but not human-invokable; `deferred` = not sent to the model until activated this session.
- [ ] Prompt-build catalog = `direct` ∪ session-activated `deferred`; ToolHost enforces at call time (a non-visible call fails with `tool_not_active`, never silently).
- [ ] `tools/activate {ids}` / `tools/deactivate {ids}` RPC (session-scoped); Journal `tools.exposure_changed` for durability across resume.
- [ ] Config: `tools.exposure.<id>` static levels + `deferred_tools: [...]` default list (low-frequency tools like `sequential_thinking`, `job_*` are the candidates — pick with a comment).
- [ ] MCP: per-server `tool_exposure` glob patterns → exposure on discovered tools (schema-hash rule unchanged).
- [ ] Tests: deferred tool absent from captured model request; activate → present in next turn's catalog; hidden tool callable internally but rejected from model path; exposure survives session reload via Journal replay; MCP annotation mapping.
- [ ] `go test ./internal/toolhost ./internal/tools ./internal/runtime ./internal/mcphost -run 'Exposure|Activate'`; `just ci`.
- [ ] Commit `feat(tools): exposure levels with session activation`.

## Boundary

Not a middleware; no runtime tool *registration* (modules still compile-time). `tool_search` consumes `deferred` catalog — built in E3.

## Acceptance

A deferred tool is invisible to the model, becomes visible after `tools/activate`, and the Journal records the change; config-level exposure applies at startup.
