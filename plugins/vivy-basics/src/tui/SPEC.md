---
id: basics-tui
type: submodule-design
status: draft
title: tui (`plugins/vivy-basics/src/tui`)
parent: basics
---

## Responsibility

The compact terminal view: the Vivy welcome and window title, a footer that stays
visible while typing (activity, Plan, model, reasoning, available usage, and the
shared workbench/provider status slot), readable tool calls with their full output
and failure causes, complete file diffs, and the Plan review surface.

## Boundary

- It composes Rig's native terminal widgets; it adds presentation, not execution.
  Editor, completion, scrolling, session controls and key bindings stay Rig's.
- Selected by the recipe's `terminal` component. Removing it excludes `rig-tui`
  entirely; the default identity still works in print and JSON mode, and
  configuration paths and `RIG_HOME` are unaffected.

## Invariants

- The footer is visible while typing, and narrow windows keep activity and Plan
  first — the view degrades by dropping detail, not by hiding that work is running.
- The right footer slot rotates between telemetry and workbench/memory status;
  infrastructure and preparation errors pin it until resolved. A recipe without
  Python has no Python detection or rotation.
- Presentation changes are branding only: no tool permission, no execution path and
  no configuration path depends on them.
- `NO_COLOR` disables colours.

## Limitations

- Not verified in this checkout beyond the scripted `scripts/branding_tui.py` and
  `scripts/plan_tui.py` runs on a pseudoterminal.
