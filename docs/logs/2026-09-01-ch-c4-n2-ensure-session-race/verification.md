# Verification

| Command | Result |
|---|---|
| `gofmt -l internal/channelhost` | clean |
| `go vet ./internal/channelhost` | clean |
| `go test ./internal/channelhost -run 'TestEnsureSessionConcurrentSameChat\|TestConcurrentInboundSameChatDispatch' -count=5 -race -v` | 10/10 PASS (9.2s) |
| `just ci` | (see below) |

## just ci

Full gate green: fmt-check, go vet, go test ./..., headless-compile,
plugin-ci, ui tsc + eslint + vitest + vite build.

## ui-e2e skip reason

Test-only slice in `internal/channelhost` — no UI, RPC, or HTTP
transport change; the browser suite has no surface to observe.
