# G1 — Loadable JSON themes with auto light/dark

Story: `docs/superpowers/plans/vivy-code-parity/g1-tui-themes.md`
Commit: `feat(tui): loadable JSON themes with auto light/dark`

## What shipped

- New `sdk/tui/theme` package: `Colors` (13 roles: primary, secondary, fg,
  muted, subtle, success, warn, danger, user, on_primary, code_bg, string,
  link), embedded `themes/dark.json` + `themes/light.json`, and
  `Resolve(dir, name)` which never errors — every failure mode degrades with
  a warning list.
- Resolution order per name: user file `<dir>/<name>.json` → embedded →
  dark fallback. `""`/`"auto"` detects the terminal background: OSC 11 query
  on `/dev/tty` with a 150 ms deadline (BT.601 luma), then `COLORFGBG`, then
  dark. Partial themes fill missing roles from the dark baseline; `#rgb`
  expands to `#rrggbb`; unknown keys/roles warn; bad hex or bad JSON warns
  and falls back — never a startup crash.
- `Palette` carries `Colors`; `PaletteFromColors` is now the sole
  roles→styles mapping (all `palette*` constants deleted). Markdown styles
  (`markdownStyle`, `quietMarkdownStyle`), glamour renderer caches
  (`mdRendererKey.theme`, `streamEntryKey.theme`), streaming markdown, and
  `renderMessageBody` are parameterized on `theme.Colors` so a theme switch
  can never poison renderer caches (`Colors.ID()` keys caches by name or
  value hash).
- Wiring: `config.TUI.Theme` (yaml `theme:`), `--use-theme`/`--no-themes`
  flags, `faceport.Options.UseTheme/ThemesDir/NoThemes`, `view.Options`.
  Flag wins over config; themes dir = `<shared settings dir>/themes` for
  `vivy-code` and `<data dir>/themes` for `vivy`/`vivy tui` remote attach.
  Invalid/unknown themes surface a startup overlay warning via the new
  `vivy.tui.dialog.theme` catalog key (en/zh).

## Accepted delta vs pi

- No theme hot-reload in v1: a theme change takes effect on restart.
- `--theme` (pi's list-submission flag) still only records paths; theme
  *files* are the supported surface.

## Files

- New: `sdk/tui/theme/{theme.go,theme_test.go,themes/dark.json,themes/light.json}`,
  `sdk/tui/view/theme_test.go`
- Refactored: `sdk/tui/view/{styles.go,markdown.go,streaming_markdown.go,render.go,model.go}`
- Wiring: `internal/config/config.go` (`TUI.Theme`),
  `internal/codeface/launch.go`, `sdk/port/face/face.go`,
  `sdk/tui/face/face.go`, `cmd/vivy/{run.go,tui.go}`
- i18n: `sdk/tui/i18n/catalog_{en,zh}.go`
- Re-pinned: `sdk/internal/assembly/conformance_results.json` digest
  `0b3bcf23…` (internal/config changed).
