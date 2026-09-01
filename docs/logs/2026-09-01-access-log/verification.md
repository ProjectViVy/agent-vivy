# Verification

| Command | Result |
|---|---|
| `gofmt -l internal/rpc internal/app` | clean |
| `go vet ./internal/rpc ./internal/app` | clean |
| `go test ./internal/rpc -run TestAccessLog -count=1 -v` | 4/4 PASS in 2.3s |
| `just ci` | green (fmt-check, vet, test, headless-compile, plugin-ci, ui tsc/eslint/vitest/build) |
| `just ui-e2e` | 10 passed / 1 skipped (25.6s; cron-tasks pre-existing skip needs a real provider) |

## Test notes

- `TestAccessLogRecordsStatusAndDuration` — 201 response logged at INFO
  with method/path/status/duration_ms.
- `TestAccessLogWarnsOnServerError` — 500 logged at WARN.
- `TestAccessLogHealthzStaysBelowInfo` — /healthz emits nothing at INFO.
- `TestAccessLogWebSocketUpgradeLogs101` — handler asserts
  `http.Hijacker` is exposed through the middleware, hijacks, and the
  middleware logs `status=101`.

## Fixed during the slice

- First draft of the WebSocket test used plain `http.Get` on the never-
  answered hijacked connection: the test hung ~130s until the transport's
  default response deadline before passing. Replaced with an explicit
  `http.Client{Timeout: 2 * time.Second}`; suite now completes in 2.3s.
  The middleware's 101 line is written when the handler returns, so the
  assertion does not depend on the client outcome.

## just ci

Full gate green: fmt-check (cmd/internal/sdk/ui/plugins glob), go vet,
go test ./..., headless-compile, plugin-ci, ui tsc + eslint + vitest
(195 tests) + vite build.

## just ui-e2e

10 passed / 1 skipped in 25.6s (cron-tasks pre-existing skip needs a
real provider). `runtime.spec.ts` drives the real control plane on
127.0.0.1:8799 — its WebSocket dial and `/rpc/bootstrap` fetch now
traverse `AccessLogMiddleware`, proving the hijack passthrough against
gorilla's real upgrade path, not only the httptest fake.
