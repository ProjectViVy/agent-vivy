# Verification

Date: 2026-10-20

Environment: Windows, Go 1.26.4, `GOPROXY=https://goproxy.cn,direct`,
`../laputa` sibling present. `ui/dist` does not exist on a fresh checkout, so
`internal/app` was compiled against a temporary `ui/dist/.keep` placeholder
that was deleted after the run (it is `go:embed all:dist`, not a product
artifact).

## Checks

- `go test -count=1 ./internal/provider/ ./internal/modelhost/` - PASS.
  Embedded-catalog counts now assert 2 vendors / 3 endpoints / 14 models;
  metadata anchors assert the DeepSeek and SenseNova rows; the reconcile tests
  cover "deferred adapter without endpoint passes" and "supported adapter
  without endpoint fails".
- `go test -count=1 ./internal/modules/defaults/` - PASS. Two compiled
  Provider Profiles, both `SUPPORTED`; the embedded-data coverage test still
  maps every endpoint to a Profile.
- `go test -count=1 ./internal/rpc/ -run 'TestProvidersCatalogServesEmbeddedData'`
  - PASS. The catalog RPC payload carries 2 vendors / 3 endpoints and no
  deferred endpoint row.
- `go test -count=1 ./internal/app/` - 66s, two failures:
  `TestAppShutdownBounded` and
  `TestAppShutdownStopsGoalAdmissionBeforeChannelDrain/Run`, both
  `TempDir RemoveAll cleanup: unlinkat ...garden.db: The process cannot access
  the file because it is being used by another process`. Re-run in isolation:
  `TestAppShutdownStopsGoalAdmissionBeforeChannelDrain` passes;
  `TestAppShutdownBounded` still hits the same Windows file-handle cleanup
  error. These are Windows garden-database handle flakes, not provider-data
  behavior; they are reported, not waived.
- Assembly regeneration: `go run ./sdk/internal/cmd/generate-default --repo . --output internal/generated/assembly/zz_default.go`
  changed exactly one line (the `ProviderProfiles` manifest list). The
  generated file is not hand-edited.
- `git diff --check` - PASS on the staged change set.

## Not run

- `just ci` (UI gate, plugin gate, headless compile), `-race`, Docker,
  Postgres, real-provider and browser smoke. These remain candidate/PR gates.
- `ui/` typecheck and tests: no UI source changed; the provider catalog is a
  backend projection.
