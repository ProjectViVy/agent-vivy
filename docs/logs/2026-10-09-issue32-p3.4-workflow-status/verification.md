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

## UI, identity, and integrated package evidence

- `pagination-status.test.tsx` and the Module workflow suite passed: 5 files / 38 tests, covering both panes' continuation, terminal cursor, error retry with retained rows, refresh replacement, stale response fencing, deduplication, busy state, native terminal cancellation, recovery-event refresh, resume-false capabilities, and start-intent behavior.
- Full UI suite passed: `vitest run` — 80 files / 620 tests. Existing React boundary and `act()` fixture diagnostics were emitted; no test failed.
- `go run ./sdk/internal/cmd/source-hash plugins/vivy-workflow e98c354a4d2522cd3b99290553303d89badd1731d1fc2217a1a51c1922e6c7d2` produced the same source identity on repeated runs. The declaration and `module.go` carry that digest; generated assembly was produced by `scripts/stage-ui-assembly.mjs`.
- `tsc --noEmit -p tsconfig.json` passed after staging the Module assembly.
- With `VIVY_POSTGRES_TEST_DSN=postgres://postgres@127.0.0.1:55432/postgres?sslmode=disable`, `go test ./internal/storage/sqlite ./internal/storage/postgres ./internal/runtime ./internal/rpc -count=1` passed all four packages; PostgreSQL executed against the disposable local instance.
- `git diff --check` passed for the combined P3.4 changes.

## Remaining verification

Aggregate `just ci`, SDK/Port conformance and the split-browser product smoke (including author/edit/publish/start retry, tied-time pagination, completed and recovery-required Runs, and invalid-cursor refresh) remain P7 gates. Native/candidate acceptance remains open; these local engineering checks do not claim product acceptance.

## Cancellation-status regression discovered during P6 verification

The broad runtime run exposed a timing-sensitive expectation in
`TestINOFYWorkflowCancelPropagates`. The pinned INOFY `Program.Run` classifies
context cancellation as `cancelled` when it can commit the interrupted node's
settlement; if cancellation also prevents that commit, its journal reports
`recovery_required`. The test fixture races cancellation with that commit, so
both outcomes are valid. The regression now accepts either engine status while
still asserting a non-terminal native Run and rejecting duplicate replay. The
test passed three repeated focused runs and the full runtime package; no product
status handling changed.
