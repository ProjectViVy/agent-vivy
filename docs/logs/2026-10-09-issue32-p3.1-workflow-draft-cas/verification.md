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

## UI red and green evidence

- Before the implementation, four browser assertions failed as intended: null did not send explicit create intent; an empty edit token was accepted; two missing-draft editors let the second create attempt proceed without the expected conflict contract; and editing a published revision did not load/use the current draft ETag.
- After implementation, direct local Vitest execution passed `face-bridge.test.ts` and `workflow-page.test.tsx`: **2 files, 21 tests passed**. Invocation used the installed Vitest binary directly because the `pnpm exec` wrapper attempted network lock validation in this environment; a temporary ignored dependency link was removed by a shell trap.
- Rehash command `go run ./sdk/internal/cmd/source-hash plugins/vivy-workflow 2247d763d31ebd397ea322df97494a7b2a006fcb28802c38e263dd592a1d3656` emitted the same hash.
- `node scripts/stage-ui-assembly.mjs` and `tsc --noEmit -p tsconfig.json` passed after staging the generated assembly through the supported script. The generated diff changes only the workflow Module source hash.
- The complete P3.1 package command `go test ./internal/storage/sqlite ./internal/storage/postgres ./internal/runtime ./internal/rpc -count=1` passed with the disposable PostgreSQL DSN configured; each of the four packages reported `ok`. The runtime child-process tests used `/workspace/.vivy-cloud/tools/go/bin` first on `PATH` so child `go version` resolves the Go compiler rather than the unrelated system `Go` board-game command.
- `git diff --check` passed.

## Remaining verification

Aggregate `just ci`, the program's SDK/Port conformance and the split real-browser acceptance path remain P7 work. No native application or release acceptance is claimed here.

## Ruling

The unavailable `oil-frontend` skill is handled under the explicit user-directed continuation ruling recorded in P3-workflow.md. The implementation is limited to planned behavioral contracts and stays within existing UI Module patterns. A later Oil-specific accessibility or visual review may still request small adjustments.
