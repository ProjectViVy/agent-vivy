# Verification

Initial scope was enumerated using the connected GitHub PR search and fetched branch heads. Git fetch confirmed main and all four submitted heads. #49 had two textual conflicts: go.mod and historical conformance source hashes. Semantic resolution is documented in summary.md.

Pre-merge full CI was deliberately deferred per explicit user instruction. Consolidated CI and exact-commit Actions follow the merged candidate.

## Sequential merges

- #49 merged by normal GitHub merge at `794c4bb2216b9572728119450a8b6b59c2591961`, with conflict-resolution head `5be6f3ec3741abf089f206e7f205e5334e2d7a69`. Bootstrap, formatting, selected queue/observer/diagnostics/literal-SQLite tests, BML initialization selection, and recall-index deletion regression passed. An initial command incorrectly addressed the independent BML module as a root package; rerunning in bml/ passed. No source digests were refreshed.
- #50 merges main cleanly and preserves the observer/cognitive shutdown repair in App.Run. Deprecated Embedded Host/C ABI and its pack paths remain deleted. SDK Generation tests passed; consolidated full gate remains pending.

- #50 normal merge succeeded at `906ab676ba4fbbf81e904bd03671bb6936d71271`; integrated head `e9b5d1162690e9d94cda72244e5df82dc68e8438`.

## PR47 database regression

The new frozen-release upgrade test failed before correction: both dialects' 017 hashes had changed; SQLite released heads 17/23/35 rejected checksum drift; repairing a missing cron table at head16 failed in 039 with a duplicate revision column. After restoring released 017, migration and full SQLite/PostgreSQL storage suites passed on real PostgreSQL 17 in a disposable local container. The original SQLite repair test initially failed because it retained marker039 while deleting the cron table; it now seeds the actual historical head16 and exercises the full additive upgrade. Both backend publication tests additionally preserve an anchored user annotation after promoted and candidate regenerations. Focused report/notebook/runtime checks and consolidated gates follow.

Focused `go test -tags vivy_headless ./internal/app ./internal/runtime ./internal/modules/notebook ./internal/modules/reports -run 'Test(Report|Notebook|ServiceDoesNotInjectNotebook|QueueReplay|Observer.*Settlement)' -count=1`: PASS. This includes report observer exclusion, notebook authority, report scheduling/admission/execution, no ordinary notebook injection, and retained queue/observer behavior. The notebook/reports module packages compile with this selection but contain no matching test names. Both real-backend annotation regressions pass. Formatting and merge diff check pass.

- #47 normal merge succeeded at `37bb044b5cd4efd560cedf3a145860a500a88612`, from integration head `8ec2dc9ce993b76fac0cd5e0200174bf1fe75308`.

## PR52 reconciliation

The child freeze handoff was requested before reconciliation. Its submitted head remained `aaaea752023b90ee40a5985b9bd89325fc6420ff` on repeated Git reads. Work was prepared on an isolated integration branch; publication uses a normal descendant push preserving every submitted commit and rejects concurrent divergent pushes. No uncommitted child work is present in this execution environment.

Conflicts were limited to the conformance case table and Pack's post-build check. Retain live semantic conformance without authored digest columns; retain `catalog.VerifyUnchanged()` but remove deleted shared-library/ABI staging. Auto-merged defaults retain #49 pure metadata imports and #47 notebook/report inventory. Both DIVA recall and memory-loop gates remain. UI staging/build, complete defaults/assembly suites, and static Verify/StageUI regressions passed. An initial assembly test preceded ui/dist creation and failed the embed precondition; after the supported UI build it passed. Go VCS stamping ignored the integration worktree's `.git` file and discovered the empty ancestor `/workspace/.git`; `go build -x` confirmed that directory selection. Final validation uses a standalone fresh clone with a real `.git` directory, without disabling VCS stamping. Unsandboxed task-owned `/var/tmp` fixtures also avoid injected ancestor instruction-discovery roots.

- #52 normal merge succeeded at `81e7ba059ce478f3267010113abcd3c84fec8423`, from integrated head `9cb83d65fdbe36a38d4aa729dcabd66951727888`. Parent subsequently confirmed that the child was frozen with a clean checkout at the initially submitted head and no outstanding uncommitted work.

## Consolidated candidate checkpoint

On `81e7ba059ce478f3267010113abcd3c84fec8423`, an independent standalone fresh clone passed `just setup` and cold `just dev -NoBrowser` without inherited Laputa, ui/dist, or node_modules. A split :3015 browser/client smoke verified health, settings, WebSocket proxy and SQLite session CRUD. Process teardown terminated the owned startup processes after the observation window.

The complete local `just ci` run passed bootstrap, formatting, UI type checking, all 628 UI tests, build, i18n and crossface checks, vet and the root backend packages through RPC/runtime/storage. Real PostgreSQL 17 was enabled. The remaining SDK/plugin/DIVA gates were still running when this checkpoint was recorded; this is not a full-green claim. Exact-main Actions run [38091558446](https://github.com/ProjectViVy/agent-vivy/actions/runs/38091558446) passed UI CI and full UI browser smoke; backend CI remained running.

Explicit SDK verify, minimal/default pack, inspect-artifact and embedded `--inspect-generation` passed. Minimal contained 7 modules; default contained 40. The first manual binary inspection used an unsupported argument and started the service; it was stopped and rerun using the documented flag. A first queue browser attempt encountered that owned listener; after cleanup, the real split :3015 queue browser regression passed.

## Cancellation fixture CI repair

The source-hash cleanup owner reported an intermittent nil-stream RPC panic in an otherwise passing validation run. Independently, a temporary deterministic probe released `holdCancellationModel` before cancelling its context and reproduced `(nil, nil)`, violating the stream contract. Several real lifecycle test teardowns release that barrier before `CancelAll`, so the adapter can dereference a nil stream. The fixture now waits for actual context cancellation as well as barrier release. Existing cancellation and lifecycle assertions remain unchanged; production behavior and test gates are unchanged. The temporary diagnostic probe was removed. Five complete RPC suite repetitions passed (94.953s); focused race checks and the complete final candidate gate follow.
