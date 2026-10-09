# Verification

## Red evidence

- The new storage codec tests initially failed to compile because the cursor helpers and sentinel did not exist.
- SQLite and PostgreSQL conformance runs showed timestamp ties were skipped, a valid continuation from a missing boundary was rejected, and the `team:flow:1` definition cursor was rejected.
- `TestINOFYListRejectsMalformedCursor` observed the legacy run cursor `1000` return an empty successful page and the malformed definition cursor become `storage_failed`.

## Green evidence

- `TestWorkflowRunCursorRoundTrip`, `TestWorkflowRunCursorRejectsInvalid`, and `TestWorkflowDefinitionCursorFinalColon` pass, including version, JSON, base64, size, unknown-field, range and final-colon cases.
- SQLite and PostgreSQL `RunPagingWithTimestampTies`, `RunPagingFromMissingBoundary`, and `DefinitionPagingWithColonIDs` pass. PostgreSQL ran against the disposable local PostgreSQL 16 instance rather than skipping.
- `TestINOFYListRejectsMalformedCursor` passes for both list RPCs and asserts `InvalidParams`, `data.code == "invalid_input"`, and a refresh message.
- Full affected package run passed: `go test ./internal/storage/sqlite ./internal/storage/postgres ./internal/runtime ./internal/rpc -count=1`, with `VIVY_POSTGRES_TEST_DSN` set.
- `git diff --check` passed.
