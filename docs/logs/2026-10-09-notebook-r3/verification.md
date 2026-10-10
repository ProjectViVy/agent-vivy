# R3 verification

## Focused suites (sqlite, plus real Postgres where dialect-bound)

- `go test ./internal/runtime -run 'TestReportSettingsWrite|TestReportCron|TestReportSkippedWindows|TestReportGetSurfacesSkipped' -count=1` — PASS (1.6s): write validation matrix (stale revision, invalid timezone, disabled without schedule, foreign/deleted section, unknown model, missing actor/op-key), replay convergence, typed `TriggerCron`/due-loop dispatch to `startReportCron`, single committed Run on double-fire and restart re-fire, manual-overlap busy, skipped-window enumeration bounded and durable, omission → `ErrWorkflowRecoveryRequired`.
- `go test ./internal/actionhost ./internal/modules/reports ./internal/rpc` — PASS: `WriteSettings` binding (op-key required, AdmissionPort delegation), module `settings.write` provider schema, `updateCron`/`deleteCron` rejection of report rows.
- `VIVY_POSTGRES_TEST_DSN=postgres://postgres:vivytest@127.0.0.1:55432/vivy_test?sslmode=disable go test -count=1 -run 'TestReportSettings|TestReport' ./internal/storage/sqlite ./internal/storage/postgres` — PASS on both dialects, no skips: `TestReportSettingsCommitReceiptReplay` exercises `CommitReportSettings` commit → receipt, same-key replay, divergent-payload `ErrIdempotencyConflict`, stale-expected `ErrRevisionConflict` on real Postgres (vivy-pg container).

## Full gate

- `just ci` — GREEN end to end (exit 0): gofmt sweep, UI gate (tsc + 625/625 vitest + vite build + i18n completeness + cross-face), `go vet ./...`, `go test -timeout 35m ./...` (all packages incl. `internal/app` baseline inventory, embedded, eval air-gap, sdk/host/v1), `sdk/internal` 331s pack/Inspect/conformance suite, plugin-ci sweep.
- Pin evidence: `vivy-module.yaml`/`module.go` → `c97a3fd5b1ce6317dec5e95a27e56e48f96c07610cde413f748b1d9c3d01256a`; `conformance_results.json` internal digest → `5dd345e42cdcfe85d1cc191cc6078ca7a8dde2975e286db4dc8cd8aa179d3c40` (recomputed by `source-hash`, 5 suites updated); `zz_default.go` regenerated via `generate-default`, not hand-edited.

## CI iterations seen

1. `vivy.reports.settings.write` outside the `vivy/reports` allow-list (embedded/host/eval compose failures) → regenerated `zz_default.go`.
2. `TestCheckedInProviderConformanceMatchesExecutedSuites` digest drift → re-pinned internal digest.
3. `TestDefaultGenerationBaselineInventory` stale baseline JSON → added `vivy.reports.settings.write`.
4. `CommitReportSettings` had no direct test → added dialect-parity test before claiming PG parity.
