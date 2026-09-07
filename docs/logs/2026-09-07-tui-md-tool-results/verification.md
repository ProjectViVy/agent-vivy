# Verification — TUI-MD-TOOL-RESULTS

Commands run from the repository root (Git Bash, Windows):

```text
go test ./sdk/tui/view
# ok  agent-vivy/sdk/tui/view  0.924s

gofmt -l sdk/tui/view
# (no output — clean)

just ci
# exit code 0 (full gate: go build ./... , go test ./... , ui checks)
```

Notes:

- The chroma-presence assertions initially looked for truecolor background
  escapes (`\x1b[48;2;`); the pipeline actually emits chroma's 256-color
  foreground escapes (`\x1b[38;5;N`, e.g. `\x1b[38;5;254m# Report`) and no
  background escape. The assertions were corrected to the foreground
  signature; the plain-path negative assertion holds because plain
  `wrapText` output carries no ANSI at all.
- No manual TUI smoke was run: the change is render-only inside tool-card
  bodies and is exercised by the deterministic card-rendering tests above;
  visual review follows the 真机评审 recommendation recorded for
  `docs/logs/2026-09-07-tui-chat-body-polish/verification.md`.
