# MR-7: verification and human-authored PR

## Steps

1. Recompute the `internal/` source digest after all internal edits and fold it
   into `reproduction_test.go` + `conformance_results.json` (last write wins;
   any later `internal/` change invalidates the pin).
2. `go build ./...`; `go test ./...` — 74 packages, zero failures.
3. `go test ./sdk/internal/conformance/` — re-executes all 23 provider suites
   (including per-plugin `go test` and the UI vitest surface) and byte-matches
   the regenerated artifact.
4. Open the PR on a `devin/*` repair branch, authored as `mastwet` per the
   repository identity rule; never push to `main`.

## Evidence

See `docs/logs/2026-09-28-merge-repair/` for the verification log.
