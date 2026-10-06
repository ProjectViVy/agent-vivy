# G1 verification

## Commands run

```text
go build ./...                          # green
go test ./sdk/tui/... ./internal/config/... ./internal/codeface/... -count=1
go test ./sdk/internal/conformance/ -count=1
```

## Results

- `sdk/tui/theme` — 10 tests pass: embedded defaults resolve, user file
  overrides embedded, custom named theme, unknown name → dark + warning,
  invalid user file → dark/embedded + warning, `#rgb` normalization +
  unknown-key warning, bad hex/JSON rejection, `Colors.ID` distinction,
  COLORFGBG fallback table (0;15 / 15;7 / 0;8 light; 15;0 / 7;0 / empty /
  garbage dark; OSC 11 skipped when no controlling terminal answers),
  `parseOSC11`, `DefaultDir`.
- `sdk/tui/view` — all pre-existing tests pass plus
  `TestPaletteFromColorsMapsEveryRole` (custom colors reach palette styles;
  `DefaultPalette` == embedded dark) and `TestNoHardcodedColorsInView`
  (scans every view/*.go for `"#hex"` literals — none remain).
- `internal/config`, `internal/codeface` — green.
- Conformance — green after re-pinning the internal digest to
  `0b3bcf2350843d6da2677dff4d21a3322d5636a5548743f899d2aad8187c4afb`
  (required by the `internal/config` TUI.Theme addition).
- `just ci` deferred to H1 per plan.

## Manual greps

- `rg '"#[0-9a-fA-F]{3,8}"' sdk/tui/view/*.go` → only markdown prefix
  literals (`"## "`), no color literals remain.
- Old `palette*` constants deleted; `PaletteFromColors` is the single
  roles→styles mapping.
