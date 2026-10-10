# VCP-F2 verification

- `go test ./internal/runtime/ -run 'CacheWarm' -count=1 -v` — 4 tests:
  streaming fires after settle (fake provider counts 1 warm call,
  payload carries prompt=111 / cache_write=1234), idle fires at
  0.8x lifetime, savings floor skips (`below_min_savings`), off mode
  makes zero extra calls and emits no event; streaming test also
  asserts warm stays unobserved (model.request count == real calls) and
  the marker never lands in stored messages.
- `go test ./internal/provider/ ./internal/config/ ./internal/domain/ -count=1` — green
  (event vocabulary bumped to 65).
- `go test ./internal/runtime/ -count=1` — green, 52s.
- `go test ./internal/app/ ./internal/rpc/ -count=1` — green.
- `go test ./sdk/internal/conformance/ -count=1` — digest re-pinned to
  `802e9273…`, green (90.8s).
