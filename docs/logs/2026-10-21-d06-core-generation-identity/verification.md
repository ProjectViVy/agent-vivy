# Verification

Source baseline `398fe41be081083d4e34d537d7ea2ef77b85e51a`, isolated branch
`codex/issue91-d06-core`. Independent Linux checkout:
`/home/mastwet/work/vivy-issue91-d06-core/agent-vivy`, with its own pinned Laputa
`ff3936f44ff8cf08c12af2cf698c194cfe474fd3`. Go1.26.4, Node24.21.0, pnpm10.33.0.

## Passed

- RED: `go test ./internal/storage/migrations -run TestGeneration -count=1`
  failed because both fresh and copied target schemas retained artifact_sha256.
  GREEN after paired037 and Generation preflight.
- `go test -tags=vivy_headless ./internal/storage/migrations ./internal/storage/sqlite ./internal/studio ./internal/eval ./internal/rpc -count=1 -timeout 8m`:
  migrations, SQLite, Studio and eval packages PASS. RPC has the separate stale
  shell-history assertion detailed below; this combined command was not green.
- Final `go test -tags=vivy_headless ./internal/storage/migrations ./internal/storage/postgres -run 'TestGeneration|TestApplyPreflight' -count=1 -v -timeout 3m`
  with the explicit disposable PostgreSQL DSN: PASS, no skips. Covers fresh and
  reopen, copied035, exact lifecycle history, source preservation, failed037
  transaction rollback, final retired-column refusal, baseline drift and
  unsupported intermediate036 refusal.
- Real PostgreSQL `TestToolOperationCopiedBaselineConversionPostgres` and
  `TestMigrateUpgradesV14InPlace`: PASS, no skips; existing finite paths reach037.
- Generation RPC subset `TestStudioRPCEmptyListAndPromote`,
  `TestSpeciesInspectBuiltinThenPromoted`, `TestGenerationsRejectRPC`,
  `TestEvalsStartUnknownSuiteAndFailedProbe`: PASS, four actual tests.
- `go vet -tags=vivy_headless ./internal/storage/migrations ./internal/storage/sqlite ./internal/storage/postgres ./internal/studio ./internal/eval ./internal/rpc`: PASS.
- `pnpm --dir ui install --offline --frozen-lockfile --ignore-scripts`: PASS
  using isolated dependencies, no downloads or lockfile changes.
- `pnpm --dir ui typecheck`: PASS, including actual SDK default UI staging with
  explicit `authoring-default` identity and TypeScript compilation.
- `pnpm --dir ui exec vitest run src/lib/store.test.ts`: PASS,65 tests, no skips.
- Go formatting and `git diff --check`: PASS.

The task-owned PostgreSQL16 container was `vivy-d06-core-postgres-20261021`,
loopback port55406, with disposable database/schemas only. It exited cleanly
between runs, causing one connection-refused attempt. Restarting that same
owned instance and running the final targeted checks passed; the container was
removed afterward. Credentials are not recorded here.

## Remaining integration evidence

The full RPC run failed only `TestControlShellStartIsGovernedAndStrict` at
control_test.go:800: it expected `bash script [redacted`, while the integrated
D03 runtime emitted the safe label `bash (script redacted)`. The raw command
`printf rpc_shell_ok` was absent from history. This lane leaves that assertion
unchanged; the lead owns the test-only integration reconciliation and final
full affected-suite rerun. The initial eval tests lacked UI assets; their real
health-probe helper now builds with the supported vivy_headless tag.

The lead owns exact integrated-candidate scoped gates, full SDK/Recipe matrix,
conformance reproduction, browser walkthrough and native Windows acceptance.
This lane does not claim those gates or publish an upgrade/release artifact.
