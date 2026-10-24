# G2 — Configurable keybindings

**Goal:** `keybindings.yaml` maps named actions → chords; all hardcoded TUI keys become named defaults.
**Epic:** G. **Requirements:** RQ-TUI.
**Spec:** [VCP-D1](../../specs/2026-10-06-vivy-code-parity-design.md) §5.8. **Baseline:** `f34f3ce`.

## Scope

**Files:** `sdk/tui` input layer — build an action registry (`action → default key`), yaml loader (`~/.vivy/keybindings.yaml`), conflict handling. Inventory current hardcoded keys first (composer, transcript nav, palette, queue keys incl. the new steer/follow_up chords from B2).

## Tasks

- [ ] Enumerate current keys into `defaultBindings` named actions (send, follow_up, steer_send, dequeue, cancel, palette, model_cycle, tree, search, copy_last, ext_editor, page nav, session picker…).
- [ ] YAML `action: chord` overrides; unknown action → warning; duplicate chord → first action wins + warning.
- [ ] `/hotkeys` command prints effective bindings (pi parity).
- [ ] Tests: override applies; conflict resolution deterministic; all defaults documented.
- [ ] `go test ./sdk/tui/...`; `just ci`.
- [ ] Commit `feat(tui): configurable keybindings`.

## Boundary

Face-internal; platform chords (Windows ctrl+q vs alt+enter) per pi's convention table where sensible.

## Acceptance

Changing `search: ctrl+f` in yaml remaps the action; `/hotkeys` shows the effective map.
