# Verification — TUI-CMD-N4

| Command | Result |
|---|---|
| `gofmt -l sdk/tui/live sdk/tui/view` | clean (no output) |
| `go test ./sdk/tui/live ./sdk/tui/view` | `ok agent-vivy/sdk/tui/live`, `ok agent-vivy/sdk/tui/view` |
| `just ci` (repository root, background run) | completed, exit code 0 |

First `go test` run surfaced one failing pre-existing assertion
(`TestDynamicCommandSerializesAndRestoresDraftAcrossCancellation` expected the
now-unreachable `submitInput` guard text); the test was updated to assert the
locked behavior and the suite re-ran green before `just ci`.

## Notes

- Windows: `just ci` runs buffered in the background; its completion was
  confirmed by the task notification (exit code 0) rather than streaming
  output.
