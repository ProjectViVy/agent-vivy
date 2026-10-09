# Verification

## Red evidence

- Shared SQLite conformance failed on repeated timestamp writes: writes two through four reused the same ETag at `UpdatedAt=1000` and after the clock moved back to `999`.
- `TestWorkflowProductSaveRequiresExplicitCAS` failed because an empty expected ETag implicitly created a draft.
- `TestINOFYSaveDraftExplicitCreateOrETag` rejected none of the omitted/null/empty edit tokens and did not reject ambiguous create-plus-ETag; duplicate explicit-create conflict shape was not available.
- With a disposable local PostgreSQL 16 container bound only to `127.0.0.1:55432`, `TestWorkflowDraftConcurrentAbsentInsertConflict` observed PostgreSQL SQLSTATE `23505` instead of `ErrWorkflowDefinitionConflict`. The legacy-token reopen test passed on the pre-change implementation.

## Green evidence

- SQLite ETag rotation, empty-token rejection and legacy-token reopen tests passed.
- Runtime explicit-CAS and RPC explicit create/edit mode tests passed.
- PostgreSQL legacy-token reopen and concurrent absent-insert tests passed against the disposable container. The losing insert maps to `ErrWorkflowDefinitionConflict`, and `errors.As` still reaches the underlying `*pgconn.PgError` with SQLSTATE `23505` and the draft primary-key constraint.
- `gofmt` and `git diff --check` passed.

## Remaining verification

Backend package coverage now passes in commit `64a3196c` as part of the P3.1-P3.3 backend group. P3.1 browser tests, editor behavior, Module source rehash, UI typecheck, `just ci` and the real Vite path remain pending; no UI acceptance is claimed yet.

## Ruling

Backend regressions and implementation were completed while the UI-specific required `oil-frontend` skill is unavailable. This keeps independent Storage/Runtime/RPC work moving; the risk if this sequencing is wrong is that later editor behavior may require a server-contract adjustment. No UI code was changed under this ruling.
