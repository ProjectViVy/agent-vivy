# Verification

## TK-01

- RED: `go test ./internal/bundlecontract -run 'Test(DecodeManifest|BundleIDs|ManifestPaths)' -count=1` failed for missing contract symbols.
- RED: digest/native Skill test command failed for missing digest/parser symbols.
- RED: `go test ./internal/bundlecontract -run TestView -count=1` failed for missing immutable View.
- GREEN: `go test ./internal/bundlecontract ./internal/bundles -count=1` passed.
- `go vet ./internal/bundlecontract ./internal/bundles` passed.
- `git diff --check` passed.
- Golden digest independently derived with Python hashlib/json/struct:
  `ac57d0d77f4eedfd11049d85a9f87be5c4b49d030cfd98c94a2755ea0f96a64b`.
- Full Windows CI pending; focused checks are not full-product acceptance.

## TK-02

- RED: `go test ./internal/bundles -run 'Test(Acquire|ObjectRecovery)' -count=1` failed for missing acquisition APIs.
- RED: `go test ./internal/bundles -run TestGitAcquisitionHasNoExecution -count=1` failed before Git extraction was implemented.
- GREEN: `go test ./internal/bundlecontract ./internal/bundles -count=1` passed.
- GREEN: `go vet ./internal/bundlecontract ./internal/bundles` passed.
- GREEN: `git diff --check` passed.
- GREEN: Windows cross compilation of the `bundles` and `bundlecontract` test binaries passed with `GOOS=windows GOARCH=amd64 go test -c`.
- Local and pinned Git fixtures verify declared-file-only copies, source immutability, no repository-hook execution, cancellation, owner availability, and stored-byte tamper detection.
- Full Windows CI pending.

## TK-03

- RED: migration-count and backend contract tests failed before schema/store implementation: SQLite backend did not expose `BundleStore`; migration count was 40.
- GREEN: `go test ./internal/storage ./internal/storage/migrations ./internal/storage/sqlite ./internal/storage/postgres -run 'Test(Bundle|EmbeddedManifestHasCanonicalPairs|ApplyFreshAndReapplyIsNoOp)' -count=1` passed. The PostgreSQL bundle conformance case is skipped because `VIVY_POSTGRES_TEST_DSN` is unset.
- GREEN: full `go test ./internal/storage ./internal/storage/migrations ./internal/storage/sqlite -count=1` passed.
- GREEN: `go vet ./internal/bundlecontract ./internal/bundles ./internal/storage ./internal/storage/bundlesql ./internal/storage/sqlite ./internal/storage/postgres` passed.
- GREEN: Windows cross compilation of storage and bundle test binaries passed.
- GREEN: `git diff --check` passed.
- PostgreSQL live transaction conformance remains blocked: no configured DSN, PostgreSQL executable or running Docker daemon is available in this environment.
