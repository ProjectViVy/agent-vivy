# Verification

## Automated

- `go test ./internal/runtime -run 'TestWorkerModelBrokerPreservesUsageDimensions|TestMapperMapsSummarizationUsageEvent' -count=1` — passed.
- `go test ./internal/runtime -run 'Test(MapperMapsSummarizationUsageEvent|ResumeEventMapperRestoresSummaryUsageRoutesFromJournal|ServiceRecoverResumableApproval|ServiceQuestionRecoveryKeepsQuestionDistinct)$' -count=1` — passed; covers the shared approval/question restart resume path and primary/failover summary attribution through SQLite storage.
- `go test ./internal/app -run TestLegacyWorkerModelBrokerPersistsAttributedCachedUsage -count=1` — passed.
- `go test ./internal/rpc ./sdk/tui/command` — passed.
- `cd ui; pnpm typecheck; pnpm test` — passed (201 tests).
- SQLite conformance including CN-26 — passed; the Postgres package compiled and its environment-gated suite remains governed by the existing test harness.
- A broad pre-gate runtime run hit the documented unrelated `TestCronAtJobDeletesAfterSuccessfulRun` 30-second timing flake; all targeted accounting tests passed. Final `just ci` result is pending below.
- First `just ci` run — failed only because `TestTokenStatsRPCSmoke` still asserted the retired any-priced total semantics. The fixture was updated to require mixed totals to be unknown/zero while retaining the known per-model/per-session 3.5 USD assertions; its focused rerun passed.
- Pre-review `just ci` — passed: formatting, UI typecheck, 201 UI tests, production build, Go vet/full tests, headless compile, and every plugin/face module. A final rerun after the resume-route fix is recorded below.
- Post-review final `just ci` — passed after the resume-route fix: formatting, UI typecheck, 201 UI tests, production build, Go vet/full tests (including the new recovery regression), headless compile, and every plugin/face module.

## Real path

- `just vivy-code` — passed; built the independent headless-tagged terminal executable.
- `.\vivy-code.exe --help` — passed and printed the independent VIVY CODE terminal/private-instance contract. The verified binary was moved to ignored `.workspace/build-artifacts/vivy-code.exe` rather than left as a source-tree artifact.
- No new interactive surface was added in this accounting slice. The preceding token/cost UI delivery attempted the isolated VIVY CODE PTY path and recorded the Windows process-creation environment failure in `docs/logs/2026-09-05-vivy-code-token-cost/verification.md`.
