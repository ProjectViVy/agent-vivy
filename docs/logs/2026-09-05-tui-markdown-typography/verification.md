# Verification

## Automation

- `go test ./sdk/tui/view/ -count=1`: passed (including `TestRenderMessageMarkdownTypography`, user/reasoning, narrow-window/bidi, and finished-message cache).
- `just ci`: passed (fmt-check, UI typecheck, 24 Vitest files / 201 tests, UI build, Go vet, full-repository Go test, headless compile, and all independent plugin/face modules; `sdk/tui/view` and `faces/tui` both green).

## Real-path smoke

- The typography contract is pinned down by view tests (glamour `WithStyles` + TrueColor, without terminal detection): H1 removes `#`, H2 retains `##`, lists use `•`, quotes use `│`, fenced code retains `fmt.Println`, and thinking blocks use QuietMarkdown + `┊`.
- Did not read or write `data/vivy.db`, `data/demo/`, or `data/workspaces/`.
