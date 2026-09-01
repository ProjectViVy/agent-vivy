# Acceptance

- A human cannot see this directly: it is a test-infrastructure fix.
- Tell: `go test -race ./internal/rpc/` passes deterministically
  (`-count=5` verified); before the fix it reported a data race in
  `TestAccessLogWebSocketUpgradeLogs101` on an unsynchronized buffer.
