# F3 — Model UX: cycle, scoped models, save-default

**Goal:** `Ctrl+P` model cycling, `scoped_models` config set, `/model` picker saves per-project default.
**Epic:** F. **Requirements:** RQ-MDL. **Predecessor:** F1 (thinking/levels surface co-located in model UX).
**Spec:** VCP-D1 §5.7.

## Scope

**Files:** `sdk/tui` (cycle action + picker save), `internal/config` (`scoped_models: []`, `default_model` per workspace/settings overlay), `internal/rpc` (model list ordering + scope field in `model/list`).

## Tasks

- [ ] `scoped_models` config; `/scope-model` toggles current model in/out of scope; `Ctrl+P` cycles scope in declared order (skips unavailable).
- [ ] `/model` selection persists as default for the project (settings overlay, workspace-scoped).
- [ ] Picker shows thinking-level badge + scope marker.
- [ ] Tests: cycle order; persistence across launch; unavailable-model skip.
- [ ] `go test ./sdk/tui/... ./internal/config`; `just ci`.
- [ ] Commit `feat(tui): scoped model cycling and saved defaults`.

## Boundary

No virtual models/routers (O2-adjacent, deferred).

## Acceptance

Cycle flips provider+model in one chord; reopening in the same project restores the saved default.
