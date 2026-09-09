# Verification

| Command | Result |
|---|---|
| `go test ./internal/runtime/ -race -count=1 -run 'TestCron'` | ok 13.467s (including 2 new settle contract tests + the existing end-to-end canary) |
| `just ci` | Green (golangci-lint + go test ./... + UI tsc/eslint/vitest + vite build) |

`just ui-e2e` was not run: there were no UI or RPC changes (test strengthening
only in `internal/runtime`).
