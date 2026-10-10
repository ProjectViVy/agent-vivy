# G2 verification

## Commands run

```text
go build ./...
go test ./sdk/tui/... -count=1
(cd faces/tui && go mod tidy && go test ./... -count=1)
go test ./sdk/internal/conformance/ -count=1
```

## Results

- `sdk/tui/view/keymap_test.go` — 8 tests: every default chord binds the
  right action, case normalization rules, missing file → defaults + no
  warning, scalar/list overrides replace (not merge), unknown action warns
  + skipped, chord conflict → first-declared action wins + warning,
  invalid YAML → defaults + warning, nil keymap inert, `keyChord` matches
  bubbletea `KeyMsg.String()` for ctrl/alt/shift/rune forms.
- `sdk/tui/command` — registry updated to 33 commands incl. `/hotkeys`
  (parse/validate/localized descriptions green).
- All pre-existing `sdk/tui` tests pass (palette/help/gate/steering keys
  unchanged by default).
- `faces/tui` module tests green after `go mod tidy` (yaml.v3 indirect dep).
- Conformance green after re-pins: `vivy/tui` source digest `9b27d803…`
  (go.mod/go.sum moved), internal digest `cd8d7bca…` (internal/codeface
  changed).
- `just ci` deferred to H1 per plan.
