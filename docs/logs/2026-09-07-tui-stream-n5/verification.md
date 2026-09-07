# Verification — TUI-STREAM-N5

| Command | Result |
|---|---|
| `go test ./sdk/tui/stream/ -count=1` | ok 0.882s |
| `go test ./internal/rpc/ -run 'RunSubscription' -count=1` | ok 1.861s |
| `go build ./...` | clean |
| `faces/tui` 模块 `go vet ./...` | clean |
| `gofmt -l`（六个改动文件） | clean |
| `just ci` | exit 0（全部包 ok，含 plugin-ci 与 UI 构建） |

New tests:

- `sdk/tui/stream/events_test.go`: `TestDecodeRequiresSubscriptionAndRunID`
  （空 subscription_id / 空 run_id / 缺字段四种 envelope 全拒）。
- `internal/rpc/control_test.go`: `TestRunSubscriptionEmptyReplayAfterTerminalCleansUp`
  （after_seq=5 ≥ 终态 seq=1 → 订阅即释放、bus 订阅者归零）；
  `TestRunSubscriptionUnknownRunReleasesSubscription`（未知 run → 订阅即释放）。
- 既有 `inbox_test.go` 三处 `Take()` 调用迁至 `TakeWithOverflow`。
