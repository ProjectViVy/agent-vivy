# Verification

## Passed

- `go test ./internal/storage/migrations -run 'TestManifest' -count=1`
- `go test ./internal/storage/migrations -run 'Test(Apply|Runner|Legacy)' -count=1`
- `go test ./internal/storage/migrations -run 'TestPostgresLegacy' -count=1`
- `go test -timeout 20m ./internal/storage/... -count=1`
- `go vet ./internal/storage/...`
- Production source guard: no `schemaV15`, `const migrationNNN`, or migration
  `CREATE TABLE` remains in non-test Go files under `internal/storage`.

## Repository-wide gate limitations

The repository-wide `go vet ./...` attempt was blocked by the checked-in UI
embed pattern because `ui/dist` is absent:
`ui/embed.go:12:12: pattern all:dist: no matching files found`.

The repository-wide `go test -timeout 20m ./... -count=1` attempt completed all
storage packages successfully but exited non-zero for the same missing UI
artifact, missing UI dependencies (`ui/node_modules/vite` and
`lucide-react`), and one runtime test that expects a `go` executable on `PATH`.
The selected Go toolchain is available by absolute path in this workspace but
is not on `PATH`.

`VIVY_POSTGRES_TEST_DSN` was not set in this environment. The PostgreSQL
integration upgrade test was executed and passed as an explicit skip; the
static legacy mapping tests and the full storage package build/test completed.
A connected PostgreSQL service is still required to produce live v14→latest
and fresh-vs-upgraded parity evidence.

The repository-level `just ci` gate was not run in this Linux workspace because
the checked-in recipe is PowerShell-oriented and the UI dependency setup is not
available here. The underlying Go storage tests and vet gate were run directly.
