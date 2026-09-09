# Verification

## Automation

- `go test ./sdk/tui/... -count=1`: passed.
- `just ci`: passed (fmt-check, UI, vet, full-repository Go test, headless compile, plugin/face modules; `sdk/tui/view` and `faces/tui` both green).

## Real-path smoke

- The typography and Chinese-language contract are pinned down by tests such as `TestCommandPaletteHighlightsMatchesAndHidesUnselectedUsage`.
- Did not read or write `data/vivy.db`, `data/demo/`, or `data/workspaces/`.
