# Verification

## Red evidence

- The new conformance test initially failed to compile because `WorkflowRunSummary` did not expose `EngineStatus`.
- After adding the summary field, `TestINOFYListRunsSeparatesNativeAndEngineStatus` showed the RPC omitted `engine_status` and returned an empty value.
- The initial projection fixture's `workflow.admitted` limits did not match its stored effective limits. The fixture was corrected to use the shared schema-2 admission model's exact `max_nodes` and `max_attempts` values.

## Green evidence

- `TestWorkflowDefinitionContract/RunSummaryEngineProjection` passes on SQLite and verifies admitted fallback, completed/succeeded, active/recovery_required, and projection-less cancelled fallback.
- `TestWorkflowDefinitionContractPostgres/RunSummaryEngineProjection` passes against the disposable local PostgreSQL 16 container.
- `TestWorkflowProductRunBindsRevision`, `TestWorkflowProductCancelRun`, and `TestWorkflowProductHonestCapabilities` pass and assert list/detail status parity and resume remains false.
- `TestINOFYListRunsSeparatesNativeAndEngineStatus` passes and checks the two JSON fields separately.
- Full affected package run passed: `go test ./internal/storage/sqlite ./internal/storage/postgres ./internal/runtime ./internal/rpc -count=1`, with `VIVY_POSTGRES_TEST_DSN` set.
- `git diff --check` passed.

## Remaining verification

P3.4.2/.3/.6 browser pagination, completed cancellation controls, recovery badges/events, Module source hash, typecheck, real browser acceptance and `just ci` remain pending. This log records backend behavior only.
