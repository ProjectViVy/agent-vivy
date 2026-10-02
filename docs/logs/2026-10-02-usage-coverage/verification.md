# Verification — usage coverage projection

All commands run on Linux with Go 1.26.4 (`just ci` is Windows-only; the
justfile equivalents below are what `just ci`/`just ui-ci`/`just i18n-check`
invoke).

## Go

- `go test ./internal/storage/... ./internal/rpc/...` — PASS
  - `TestUsageProjectionLatestSamplePerAttempt` (10 then 15 → 15, run+seq
    determinism tiebreak)
  - `TestUsageProjectionAttemptStates` (settled/failed/cancelled/interrupted,
    zero vs nil usage counts)
  - `TestUsageProjectionActiveCall`, `TestUsageProjectionUnknownBuckets`,
    `TestUsageProjectionInvalidEvidence` (invalid sample excluded, earlier
    valid sample kept + NormalizationPartial),
    `TestUsageProjectionStartTimeSelection` (attempt start outside window
    excluded even with in-window sample),
    `TestUsageProjectionFinishUsageEvidence`,
    `TestUsageProjectionSummaryAttribution` (summary never priced as main)
  - `TestBuildTokenSnapshotCoverage`, `TestBuildTokenSnapshotCoverageStates`
    (empty/complete/partial/legacy incl. pure-legacy priced scope)
  - PostgreSQL parity suite compiles and skips without
    VIVY_POSTGRES_TEST_DSN (as designed).
- `go vet ./internal/storage/... ./internal/rpc/...` — clean
- `git ls-files '*.go' | xargs gofmt -l` — no output

## UI

- `pnpm --dir ui install --frozen-lockfile && pnpm typecheck` — PASS
- `pnpm --dir ui test` — 69 files / 522 tests PASS, incl. new
  `TokenStatsPanel.test.tsx` (partial-coverage rendering, unknown-bucket
  dashes, coalesced refresh off authoritative run events)
- `pnpm --dir ui build` — PASS
- `node scripts/check-i18n-completeness.js` — PASS (1484/1484 keys, 156
  placeholders)
- `node --test scripts/check-i18n-cross-face.test.js` — 8 pass
- `node scripts/check-i18n-cross-face.js` — PASS; new keys registered in
  `scripts/i18n-cross-face-contract.json` webKeys
- `go test -run '^$' -tags vivy_headless ./cmd/vivy ./cmd/vivy-code ./ui` —
  headless compile PASS

## Assembly digest

`go run ./sdk/internal/cmd/source-hash internal ""` recomputed after the
internal/ edits; the five internal-rooted `sourceSha256` entries in
`sdk/internal/assembly/conformance_results.json` refreshed; the two digest
gates (`TestPackAndInspectSealUIAssemblyIdentity`,
`TestCheckedInProviderConformanceMatchesExecutedSuites`) re-verified.
