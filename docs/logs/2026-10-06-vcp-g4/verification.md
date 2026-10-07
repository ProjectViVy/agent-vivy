# G4 — verification

- `go build ./...` — green (root module)
- `go test ./sdk/tui/view/ -count=1` — green (0.92s): probe table, kitty
  chunking, iTerm sequence + missing-file, `imageCellRows` sizing/cap,
  kitty transmit-once + placement + fallbacks, off + iTerm2 paths
- `go test ./faces/tui/...` — green; `faces/tui/go.mod` gained
  `golang.org/x/image` via scoped tidy
- `go test ./internal/config/ ./internal/codeface/` — green
- `go test ./sdk/internal/conformance/ -count=1` — green (65.5s) after
  re-pinning: internal digest `a6e7d5b2…`, vivy/tui digest `daa1a129…`
  (go.mod moved by x/image → new fixed point in face.go, vivy-module.yaml,
  reproduction_test.go table, conformance_results.json)
- Manual kitty/iTerm2 visual check: **untested — no graphics terminal on
  this VM** (documented per story acceptance fallback)

Not run (deferred to H1): `just ci`.
