# Verification

- `gofmt -l .` in `plugins/feishu` — clean; `go vet ./...` — pass.
- `go test -race -run TestStopDuringFirstConnectReturns -count=3 .` — 3/3.
- `go test -race -count=1 .` (whole feishu module) — ok, 4.5s.
- `just ci` — exit 0 (fmt-check, vet, go test, headless compile,
  plugin-ci covering the feishu module, UI tsc/eslint/vitest/vite build).
