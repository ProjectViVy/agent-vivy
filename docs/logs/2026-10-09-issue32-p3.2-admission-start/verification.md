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

## Execution ruling

The plan's `host_admission` diagnostic expectation conflicts with adding `uniqueItems` to the catalog schema: INOFY returns a capability `schema_mismatch` diagnostic first, identifying `tool_names` and equal items. The regression accepts that earlier fail-closed schema diagnostic. Cost if wrong: a consumer may rely on a different diagnostic code.

The server-side P3.2 work and independent P3.3 cursor work proceeded while required UI implementation guidance is unavailable. UI Module files remain untouched; the cost if this sequencing is wrong is a later browser contract adjustment.
