# Verification — VC-2 Cost accounting + model metadata (D9)

All commands run in the worktree `agent-vivy-vc0` on `feat/vc1a-bash-tool`.

## Gate

- `just ci` — PASS after the slice (gofmt check, `go vet ./...`, full Go
  test suite, UI typecheck, UI vitest 24 files / 195 tests, vite build).
- Intermediate: `go build ./internal/...` PASS during development;
  `gofmt -l internal/` clean; targeted packages rerun below.

## Package tests (new/updated)

- `go test ./internal/provider/` — PASS. New
  `TestOpenAIRefModelInfoMetadata` (gpt-4o → 128000 ctx / 2.5 / 10 /
  images=true; gpt-3.5-turbo → images=false; unknown model → all zero).
- `go test ./internal/rpc/` — PASS. `tokenstats_test.go` updated for the
  resolver parameter plus new `TestBuildTokenSnapshotCost`:
  priced gpt-4o 1M in + 100K out @ 2.5/10 = $3.5; unpriced model excluded
  from sums with cost_known=false; per-model and per-session flags;
  all-unpriced → cost_known=false overall; nil resolver → nothing priced.
- `go test ./internal/app/ -run TestTokenStatsRPCSmoke -v` — PASS (0.52s).
  Real-path smoke: full app composition on scratch SQLite, journal seeded
  through the storage layer (priced gpt-4o run with 700K cached tokens +
  unpriced custom-model run), `stats/tokens` called over the live
  WebSocket control plane (same harness as the real-provider smoke).
  Asserts on the actual wire JSON: totals (input 1,000,050 / output
  100,010 / tokens 1,100,060 / cached 700,000 / requests 2 / $3.5 /
  cost_known=true), gpt-4o share (1,100,000 tokens, $3.5, known),
  custom-model share (60 tokens, unknown), per-session costs (priced $3.5
  known; unpriced cost unknown, never $0-known).

## UI checks

- `pnpm typecheck` — PASS (worktree ui/).
- `pnpm test` — PASS (24 files / 195 tests).
- `vite build` — PASS via `just ci`.

## Smoke exception (recorded)

The live-browser panel check at `http://127.0.0.1:3015` was not run with
real usage rows: generating usage requires model calls, which require a
provider key that does not exist in this environment (same exception as
the headless slice). The data path behind the panel is instead verified
end to end by `TestTokenStatsRPCSmoke` over the real WebSocket transport;
the rendering surface is covered by typecheck + vitest + production
build. Recorded per rulebook; not a `just ci` slice skip.
