# N1 verification

- `go test ./internal/storage/sqlite -run TestNotebook -count=1` — PASS
  (AssertNotebookContract: seed/retry-replay/CAS race/scope isolation/section
  lifecycle/tombstones/comments/adopt/bounds/pagination; legacy import incl.
  >256KiB row; interrupted-migration rollback + clean re-apply; empty DB).
- `VIVY_POSTGRES_TEST_DSN=postgres://postgres:vivytest@127.0.0.1:55432/vivy_test?sslmode=disable go test ./internal/storage/postgres -run TestNotebook -count=1` — PASS (same contract + import on real PG 16, isolated schemas).
- `go test ./internal/storage/... ./cmd/vivy -count=1` — PASS incl.
  `TestNotebookExportWithoutModules` (export artifact, overwrite/traversal
  refusal, pre-036 upgrade report).
- `GIT_CONFIG_GLOBAL=/dev/null just ci` — PASS end-to-end (fmt, vet, full Go
  suite both modules, UI 598 tests, plugin-ci, sdk pack/eval).
- Source digest re-pinned: `cfd0b115…` (5 internal rows in
  `sdk/internal/assembly/conformance_results.json`).
