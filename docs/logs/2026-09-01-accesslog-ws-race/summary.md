# Access-log websocket test data race (test-only fix)

## What changed

- `internal/rpc/accesslog_test.go`: `TestAccessLogWebSocketUpgradeLogs101`
  read the log buffer from the test goroutine while the middleware wrote
  the 101 line from the server goroutine — a data race on a plain
  `bytes.Buffer` that fails under `go test -race`.
- The recorder is now a mutex-guarded `syncBuffer` (`strings.Builder` under
  a mutex with `Write`/`String`/`Len`), the same pattern used by the
  channel-plugin supervised-loop tests.
- The single-shot `strings.Contains(buf.String(), "status=101")` became a
  5s poll: the middleware logs after the hijacked handler returns, so the
  line can land after the client's doomed request returns regardless of
  the race.

## Scope

- Test file only; no product code touched. The middleware itself
  (`internal/rpc/accesslog.go`) is unchanged and unaffected: in production
  the only concurrent consumer of a slog handler is the logging backend,
  not an unsynchronized buffer.

## Not done

- No sweep for other plain-buffer slog targets in tests; the channel
  plugin tests already use guarded buffers (CH-C6-N1).
