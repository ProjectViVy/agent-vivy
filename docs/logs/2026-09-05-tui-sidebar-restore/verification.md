# Verification

## Automation

- `go test ./sdk/tui/view/ -count=1`: passed (including `TestComputeLayoutUsesBothCrushBreakpoints`; wide Markdown still retains the right pane).
- `just ci`: passed.

## Real-path smoke

- Did not read or write `data/vivy.db`, `data/demo/`, or `data/workspaces/`.
