# Verification — CMP-1 clear offload

| Command | Result |
|---|---|
| `gofmt -l` (5 files changed) | Empty (no unformatted files) |
| `go vet ./internal/runtime/ ./internal/app/` | ok |
| `go test ./internal/runtime/ -run 'TestEngineReduction\|TestSafeOffloadCallID\|TestEngineSummarizationCompaction\|TestEngineSummaryModel' -count=1` | ok 0.478s (includes new contract tests) |
| `go mod tidy` | go.mod: google/uuid changed from indirect to direct; go.sum correspondingly tightened |
| `go test ./internal/app/ -count=1 -race` | ok 14.388s |
| `go test ./internal/runtime/ -count=1 -race` (run 1) | **FAIL 149.3s** — 1 case failed; test name was not captured because the tail was truncated |
| `go test ./internal/runtime/ -count=1 -race` (run 2) | ok 102.8s |
| `go test ./internal/runtime/ -count=1 -race` (run 3, all output captured in background) | ok (exit 0) |
| `go test ./internal/runtime/ -run 'TestEngineReduction\|TestSafeOffloadCallID' -count=4 -race` | ok 2.164s (new cases stable across 4 runs) |
| `just ci` | All green: fmt-check + go vet + go test ./... + headless-compile + plugin-ci (6 modules) + UI tsc/eslint/vitest 195 + vite build |

## Notes

- The first `-race` run failed, but the failing test name was not captured (the
  `tail -5` output was truncated). The isolated new cases were all green with
  `-count=4 -race`, the following two full `-race` runs were green, and
  `just ci` was green. This is classified as an existing flaky surface (TEST-2
  tracks similar cases), with no reproduced connection to this slice's changes.
- ui-e2e was not run: this slice has no UI/RPC/WS surface (only runtime/app
  wiring and tests), so there was no trigger condition for `just ui-e2e`.
