# Verification — TUI-MD-STREAM-CACHE

| Command | Result |
|---|---|
| `gofmt -l sdk/tui/view` | clean (no output) |
| `go build ./sdk/tui/...` | ok |
| `go test ./sdk/tui/view ./sdk/tui/live` | `ok agent-vivy/sdk/tui/view`, `ok agent-vivy/sdk/tui/live` |
| `just ci` (repository root, background run) | completed, exit code 0 |

## Real-device note

This slice ports the render-cache strategy: the visible content of streaming
bubbles and the completed-state render path are exactly as before (unit tests
pin the same `renderMessage` render contract), with no keybinding or layout
changes. Following the same guidance as
`docs/logs/2026-09-07-tui-chat-body-polish/verification.md`, the Windows
Terminal real-device review should be run during human acceptance: observe
per-tick rendering updates during long-answer streaming, with no tearing
between blocks; this directory's `acceptance.md` lists the acceptance points.
