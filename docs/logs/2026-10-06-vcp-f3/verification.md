# VCP-F3 verification

- `go test ./internal/rpc/ -run 'TestModelScope|TestModelCycle' -count=1` —
  4 tests: live-selection toggle in/out (persisted + scoped flag),
  declared-order cycle across 3 models incl. wrap-around and the
  project_defaults pin written in the same transaction, unavailable-model
  skip (`invented` skipped, `gpt-5` selected), empty-set → InvalidParams
  plus `model.scope`/`model.cycle` in the initialize capabilities.
- `go test ./internal/app/ -run TestResolverProjectPin -count=1` — pin
  restores openai/gpt-4o over the global deepseek selection, twice
  (relaunch), and a different project root keeps the global selection.
- `go test ./sdk/tui/... ./internal/config ./internal/app/... ./internal/rpc/... -count=1` — green.
- `go test ./sdk/internal/conformance/ -count=1` — digest re-pinned to
  `510d2d8c…`, green (63.8s).
- Fixed during bring-up: typed-nil `*Error` returned through the cycle
  persist closure (internal error -32603), removed a leftover dead
  closure in `mapProvidersView`, and the settings-load panic guard for
  an uninitialized `project_defaults` map.
