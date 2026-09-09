# Verification — TUI-STREAM-N5

| Command | Result |
|---|---|
| `go test ./sdk/tui/stream/ -count=1` | ok 0.882s |
| `go test ./internal/rpc/ -run 'RunSubscription' -count=1` | ok 1.861s |
| `go build ./...` | clean |
| `faces/tui` module `go vet ./...` | clean |
| `gofmt -l` (six changed files) | clean |
| `just ci` | exit 0 (all packages ok, including plugin-ci and UI build) |

New tests:

- `sdk/tui/stream/events_test.go`: `TestDecodeRequiresSubscriptionAndRunID`
  (empty `subscription_id` / empty `run_id` / all four envelopes missing a
  field are rejected).
- `internal/rpc/control_test.go`: `TestRunSubscriptionEmptyReplayAfterTerminalCleansUp`
  (`after_seq=5 ≥ terminal seq=1` → subscription is released immediately and
  bus subscriber count returns to zero); `TestRunSubscriptionUnknownRunReleasesSubscription`
  (unknown run → subscription is released immediately).
- Three existing `Take()` calls in `inbox_test.go` were migrated to
  `TakeWithOverflow`.
