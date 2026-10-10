# Verification

Initial scope was enumerated using the connected GitHub PR search and fetched branch heads. Git fetch confirmed main and all four submitted heads. #49 had two textual conflicts: go.mod and historical conformance source hashes. Semantic resolution is documented in summary.md.

Pre-merge full CI is deliberately deferred per explicit user instruction. Consolidated CI and exact-commit Actions are pending.

## Sequential merges

- #49 merged by normal GitHub merge at `794c4bb2216b9572728119450a8b6b59c2591961`, with conflict-resolution head `5be6f3ec3741abf089f206e7f205e5334e2d7a69`. Bootstrap, formatting, selected queue/observer/diagnostics/literal-SQLite tests, BML initialization selection, and recall-index deletion regression passed. An initial command incorrectly addressed the independent BML module as a root package; rerunning in bml/ passed. No source digests were refreshed.
- #50 merges main cleanly and preserves the observer/cognitive shutdown repair in App.Run. Deprecated Embedded Host/C ABI and its pack paths remain deleted. SDK Generation tests passed; consolidated full gate remains pending.
