# Verification

## Automation

- `go test ./sdk/tui/view/ -count=1`: passed (including `TestFooterUsesShiftTabModeAndCtrlXShortcuts`).
- `just ci`: passed.

## Real-path smoke

- Did not read or write `data/vivy.db`, `data/demo/`, or `data/workspaces/`.
