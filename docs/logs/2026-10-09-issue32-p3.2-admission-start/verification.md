# Verification

## Red evidence

- `TestINOFYAdmissionRejectsDuplicateToolNames` accepted literal duplicate `read_file` entries. Duplicate names with surrounding whitespace and empty names were rejected only as authority-widening errors, not by canonical tool-name validation.
- The schema regression first used a `[]string` test mutation, which JSON Schema rejected as the wrong Go JSON type. After changing it to `[]any`, the test correctly failed because duplicate `tool_names` entries were accepted.
- `TestWorkflowProductPublishRejectsDuplicateTools` returned no validation diagnostic for duplicate names.
- `TestWorkflowProductHistoricalDuplicateToolsRejectStart` returned success and admitted the old publication before canonical-name validation.
- `TestINOFYStartRunRejectsMismatchedCapturedSession` received an internal error from the downstream missing-revision path instead of rejecting the mismatched captured session first.

## Green evidence

- Duplicate-name schema, canonical host validation, publication rejection, historical-start rejection, and mismatched-session RPC regressions pass.
- `go test ./internal/runtime ./internal/rpc -count=1` passes as part of the full four-package run recorded below.
- Full backend run passed: `go test ./internal/storage/sqlite ./internal/storage/postgres ./internal/runtime ./internal/rpc -count=1`, with `VIVY_POSTGRES_TEST_DSN` set to the disposable local PostgreSQL 16 instance.
- `git diff --check` passed.

## UI red and green evidence

- The new bridge tests first failed because `FaceBridge.prepareStartRun` did not exist. After adding the bridge contract but before the UI updates, editor/published UI regressions showed start requests missing captured session data and save confirmation still using stale ETags after a failed follow-up.
- Focused workflow Module tests passed: 4 files, 29 tests, including retry identity, no resave, parent changes, source conflict, immediate ETag confirmation, current-token revision edit, and same-author create collision.
- The complete UI suite passed: 79 files, 611 tests. Two existing suites print intentional React error-boundary/`act` fixture diagnostics to stderr; there were no failing tests.
- `go test ./internal/runtime ./internal/rpc -count=1` passed with the configured Go toolchain: runtime 39.598s and RPC 13.629s. The targeted runtime stale-source operation regression and captured-session RPC regression also passed.
- Module hash recomputation twice emitted `26a119336563334d0ae75a478734730caa614085794a242fc2442453e05fbd18`. The supported UI assembly staging script ran and `tsc --noEmit -p tsconfig.json` passed.
- `git diff --check` passed.

## Remaining verification

Aggregate `just ci`, SDK/Port conformance, source-bound conformance bundle refresh and real-browser product acceptance remain P7 integration work. No native application or release acceptance is claimed.

## Execution ruling

The plan's `host_admission` diagnostic expectation conflicts with adding `uniqueItems` to the catalog schema: INOFY returns a capability `schema_mismatch` diagnostic first, identifying `tool_names` and equal items. The regression accepts that earlier fail-closed schema diagnostic. Cost if wrong: a consumer may rely on a different diagnostic code.

The absent `oil-frontend` skill is handled under the user-directed continuation ruling in P3-workflow.md. The implementation follows existing Module behavior patterns; an Oil-specific accessibility or visual review may still request small changes.
