# Verification

Initial scope was enumerated using the connected GitHub PR search and fetched branch heads. Git fetch confirmed main and all four submitted heads. #49 had two textual conflicts: go.mod and historical conformance source hashes. Semantic resolution is documented in summary.md.

Pre-merge full CI is deliberately deferred per explicit user instruction. Consolidated CI and exact-commit Actions are pending.

## Sequential merges

- #49 merged by normal GitHub merge at `794c4bb2216b9572728119450a8b6b59c2591961`, with conflict-resolution head `5be6f3ec3741abf089f206e7f205e5334e2d7a69`. Bootstrap, formatting, selected queue/observer/diagnostics/literal-SQLite tests, BML initialization selection, and recall-index deletion regression passed. An initial command incorrectly addressed the independent BML module as a root package; rerunning in bml/ passed. No source digests were refreshed.
- #50 merges main cleanly and preserves the observer/cognitive shutdown repair in App.Run. Deprecated Embedded Host/C ABI and its pack paths remain deleted. SDK Generation tests passed; consolidated full gate remains pending.

- #50 normal merge succeeded at `906ab676ba4fbbf81e904bd03671bb6936d71271`; integrated head `e9b5d1162690e9d94cda72244e5df82dc68e8438`.

## PR47 database regression

The new frozen-release upgrade test failed before correction: both dialects' 017 hashes had changed; SQLite released heads 17/23/35 rejected checksum drift; repairing a missing cron table at head16 failed in 039 with a duplicate revision column. After restoring released 017, migration and full SQLite/PostgreSQL storage suites passed on real PostgreSQL 17 in a disposable local container. The original SQLite repair test initially failed because it retained marker039 while deleting the cron table; it now seeds the actual historical head16 and exercises the full additive upgrade. Both backend publication tests additionally preserve an anchored user annotation after promoted and candidate regenerations. Focused report/notebook/runtime checks and consolidated gates follow.

Focused `go test -tags vivy_headless ./internal/app ./internal/runtime ./internal/modules/notebook ./internal/modules/reports -run 'Test(Report|Notebook|ServiceDoesNotInjectNotebook|QueueReplay|Observer.*Settlement)' -count=1`: PASS. This includes report observer exclusion, notebook authority, report scheduling/admission/execution, no ordinary notebook injection, and retained queue/observer behavior. The notebook/reports module packages compile with this selection but contain no matching test names. Both real-backend annotation regressions pass. Formatting and merge diff check pass.
