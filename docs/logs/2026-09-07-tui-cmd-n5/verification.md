# Verification — TUI-CMD-N5

Commands run from the repository root (Git Bash, Windows):

```text
go build ./...
# (clean)

go test ./sdk/tui/...
# ok  agent-vivy/sdk/tui/command  (cached)
# ok  agent-vivy/sdk/tui/face     0.994s
# ok  agent-vivy/sdk/tui/live     1.024s
# ok  agent-vivy/sdk/tui/stream   (cached)
# ok  agent-vivy/sdk/tui/view     1.812s

gofmt -l sdk/tui
# (no output — clean)

just ci
# exit code 0 (full gate)
```

Notes:

- The cancellation test uses a blocking `commands/expand` handler that
  selects on the recorded call context; `CancelDynamicCommand` unblocks it
  deterministically (no sleeps).
- No manual TUI smoke: the change is interaction plumbing (Esc/session-switch
  cancellation, error-bar cleanup) exercised by the deterministic driver
  tests above; visual review follows the 真机评审 recommendation recorded
  for `docs/logs/2026-09-07-tui-chat-body-polish/verification.md`.
