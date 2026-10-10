# VCP-F3 — Scoped model cycling and saved per-project defaults

## What landed

pi's `scoped_models` cycle set, `/scope-model` toggle, one-chord model
cycling, and picker selections that persist as the current project's
default. All in the settings.yaml operator overlay (`scoped_models`,
`project_defaults`) — no new file, no config.yaml growth.

## Behavior

- `settings.ScopedModel{Provider, Model, BaseURL}` is the selection
  identity triple. `scoped_models` is the declared-order cycle set;
  `project_defaults[root]` pins a pick per project.
- `model/scope` RPC toggles a selection (the live one by default) in/out
  of the set. Settings-only write — not gated on Frozen or run state.
- `model/cycle` RPC walks the set starting after the live selection,
  skips entries whose adapter is not executable or whose model left the
  catalog, then applies the winner through the same
  `ChangeModelWhenIdle` busy-fence + persist + project-pin path as
  `settings/model/select`. Both verbs are capabilities-advertised.
- `settings/model/select` and `model/cycle` both write
  `project_defaults[<project root>]` inside the same transaction as the
  global selection; `ModelResolver` (via `newModelResolverForProject`)
  re-applies the pin on every `Current()`/`Live()` read, so a relaunch
  in the same project restores the saved model. Frozen ENV sessions are
  unaffected.
- TUI: **Alt+P** cycles (pi's Ctrl+P is taken by the command palette;
  Alt matches the steering-modifier family Alt+Enter/Alt+Up),
  `/scope-model` toggles the current model, and the picker renders
  badges: `◆` scoped member, `⌁` extended-thinking capable. The
  providers view projects `scoped_models` for ordering — scoped entries
  sort right after the current model in declared order — and
  `thinking_models` per endpoint for the badge.

## Files

- `internal/app/settings/settings.go` — `ScopedModel`, `ScopedModels`,
  `ProjectDefaults`, `Scoped`/`ToggleScoped`/`ProjectDefault`/
  `PinProjectDefault`, load normalization, `IsZero`.
- `internal/app/{model,app}.go` — `projectRoot` on the resolver, pin
  applied in `currentLocked`, wired from `NewWithAssembly`.
- `internal/rpc/control.go` — `model/scope` + `model/cycle` verbs and
  capabilities, `modelSelectionAllowed` extraction, pin in selectModel's
  persist, `thinking_models`/`scoped_models` in the providers view.
- `sdk/tui/surface` — `ModelOption.Scoped/Thinking`,
  `CycleModel`/`ScopeModel`, `ModelScopedMsg`.
- `sdk/tui/live` — view projection + client verbs + controller fence.
- `sdk/tui/view` — Alt+P chord, `/scope-model`, msg routing, picker
  badges; `sdk/tui/command` — spec + validation; `sdk/tui/i18n` — en/zh.
- `internal/rpc/control_model_scope_test.go` (new),
  `internal/app/model_test.go` — pin-restore test.
