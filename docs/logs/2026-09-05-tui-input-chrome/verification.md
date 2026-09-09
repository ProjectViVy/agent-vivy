# Verification

## Automation

- `go test ./sdk/tui/view -count=1`: passed (including `TestInputChromeUsesShiftHHelpAndKeepsKeysOnTheRight`).
- `just ci`: passed (fmt-check, ui-ci, vet, test, headless-compile, plugin-ci).

## Real-path smoke

- Did not read or write `data/vivy.db`, `data/demo/`, or `data/workspaces/`.
