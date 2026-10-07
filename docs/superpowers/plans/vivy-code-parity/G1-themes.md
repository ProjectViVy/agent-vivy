# G1 — TUI JSON themes

**Goal:** `~/.vivy/themes/*.json` + `theme: auto|light|dark|<name>`; `sdk/tui/view/styles.go` becomes a loaded theme struct; terminal light/dark auto-detect.
**Epic:** G. **Requirements:** RQ-TUI.
**Spec:** [VCP-D1](../../specs/2026-10-06-vivy-code-parity-design.md) §5.8. **Baseline:** `f34f3ce`.

## Scope

**Files:** `sdk/tui/view/styles.go` + new `sdk/tui/theme/` (loader, schema, auto-detect), `internal/config` (`theme` setting), `sdk/tui/live` (apply on start). Ship `light.json`/`dark.json` defaults embedded in the binary; user files override by name.

## Tasks

- [ ] Extract the current hardcoded palette into a `Theme` struct; JSON schema = name + color map (use hex; validate bounds).
- [ ] Loader order: embedded defaults → user themes dir → `theme:` setting → `auto` picks light/dark by terminal background (OSC 11 query with timeout fallback to dark).
- [ ] Invalid theme file → visible startup warning + fallback (never a crash).
- [ ] Tests: parse valid/invalid themes; embedded defaults always resolve; auto-detect fallbacks; no style call-site reads hardcoded colors after migration.
- [ ] `go test ./sdk/tui/...`; `just ci`.
- [ ] Commit `feat(tui): loadable JSON themes with auto light/dark`.

## Boundary

TUI only (GUI themes are CSS/std-ui-extension territory). No theme hot-reload in v1 (restart picks it up — pi hot-reloads; note as accepted delta, cheap to add later via fs watcher).

## Acceptance

Dropping a theme json into `~/.vivy/themes/` changes the palette on next launch; malformed file warns and falls back.
