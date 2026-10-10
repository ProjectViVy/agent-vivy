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

The source-hash cleanup owner reported an intermittent nil-stream RPC panic in an otherwise passing validation run. Independently, a temporary deterministic probe released `holdCancellationModel` before cancelling its context and reproduced `(nil, nil)`, violating the stream contract. Several real lifecycle test teardowns release that barrier before `CancelAll`, so the adapter can dereference a nil stream. The fixture now waits for actual context cancellation as well as barrier release. Existing cancellation and lifecycle assertions remain unchanged; production behavior and test gates are unchanged. The temporary diagnostic probe was removed. Five complete RPC suite repetitions passed (94.953s); five focused race repetitions passed (21.129s). The complete final candidate gate follows.

The combined notebook/report browser suite exposed a second test assumption: the notebook document test created a section but did not select it. After preceding report tests had populated the initially selected daily section, its row-count assertion briefly matched the existing report and its subsequent unqualified click became ambiguous when the new document appeared. Explicitly select the created section and assert it starts empty before creating its document. All document lifecycle, restart, conflict, chat-isolation and report assertions remain in place. The first combined run passed all three report tests and the notebook seeded-role test; the document failure prevented three later serial checks. The corrected combined suite must be rerun.

- #53 normal merge succeeded at `663137fec264a86441389db7dac37ec4c4d625ce`, from repair head `4e6c951036a88cc9d783f083c9f92814581b8579`. On that commit, the corrected combined notebook/report browser suite passed all 8 tests (30.3s). Cold startup/client smoke, 628 UI tests, SDK Verify, minimal/default pack and artifact/embedded/SDK manifest equality passed again. Corrupting an isolated minimal artifact's Generation ID was correctly rejected with an identity mismatch. Actions [38092303703](https://github.com/ProjectViVy/agent-vivy/actions/runs/38092303703) passed UI CI and full UI browser smoke; backend CI was still running at this checkpoint.

## DIVA recipe test selection repair

The full gate on `81e7ba05` reached the DIVA memory-loop App pass and failed compilation because the new `assembly_notebook_test.go` references `RuntimeAssembly.NotebookFactory`, a field absent from the notebook-free DIVA recipe. The notebook tests had already passed in the ordinary default-body pass. Extend the existing default-only file list to include that file when compiling the DIVA body; do not remove any `TestMemoryLoop` case. Add a guard that fails if any excluded default-only file ever contains a `TestMemoryLoop` declaration. All excluded files still execute in the first pass.

The full default Notebook selection passed separately (1.252s). The corrected complete memory-loop script, final full CI and exact-final-commit Actions follow. These checkpoints do not claim terminal full-green CI.

- #54 normal merge succeeded at `b0fac9a0091c5eb9319690e7664053035911247d`, from repair head `51572ebd193e08d8881e17924a6c850927be18d6`. Fresh cold startup/client smoke, setup/UI build, SDK verify/pack/artifact identity and tamper rejection passed on that commit. All 8 notebook/report browser tests passed (36.3s), and the queue browser regression passed (11.9s). All ordinary backend packages, real PostgreSQL, SDK assembly and live conformance passed in the complete gate before its DIVA pass. Actions [38092749854](https://github.com/ProjectViVy/agent-vivy/actions/runs/38092749854) passed UI CI and full UI browser smoke; backend remained running. The local full run was stopped after the separately executing complete memory pass revealed the next reproducible blocker.

## Memory fixture shutdown repair

The complete supported DIVA memory pass failed (905.018s), mainly with repeated fixture-close deadlines; correction/deletion also timed out while restarting, and the recall-deadline case exited with its child's cleanup failure. The fixture had begun running real `App.Run` but stored its single result in a consumable channel. Explicit child shutdown consumed it; the child's subsequent test cleanup called Close again and waited on the empty channel, causing both local and remote cleanup deadlines. This does not require changing production shutdown ordering or enlarging deadlines.

The existing `TestMemoryLoopFixtureCloseAfterRestartIsIdempotent` reproduced the failure alone (3.642s). Replace the one-result channel with a closed completion channel plus the saved Run error, so every Close observes the same completion/result. The existing idempotence, actual correction/deletion model-input and recall-deadline tests then all passed under the real DIVA overlay (24.058s), with their original deadlines and assertions. Repeated race checks and the full final gate follow.

- #55 normal merge succeeded at `1d6c9e236c11d929dd3423ed63272e88ed13e367`, from repair head `d24525a23136745c1f208ffeae3a90484f1f033e`. Five focused race repetitions passed (21.467s). On final-source capture, cold startup, setup/UI build, SDK verification and minimal/default artifact/embedded/inspection identity, explicit tamper rejection, all 8 notebook/report browser checks (32.9s), and queue browser (11.1s) passed. Complete SDK integration (309.034s), assembly (8.185s), live conformance (65.171s), ordinary App/RPC and preserved storage checks passed. Actions [38093854187](https://github.com/ProjectViVy/agent-vivy/actions/runs/38093854187) passed UI CI and full UI browser smoke; backend remained running at this checkpoint.

The complete supported DIVA memory pass then passed (767.036s), confirming the fixture shutdown repair across the suite. The next, separate six-test recall gate failed compilation because its package-based `vivy_diva_integration` overlay also omitted the direct NotebookFactory field. The default inventory tests already use `!vivy_diva_integration`; apply that same tag to the default-only notebook assertion file. Its five notebook assertions still execute in ordinary CI, and the existing memory file-selection guard remains. No memory or recall test is excluded or weakened. Focused default/recall and unaffected headless/plugin gates, then exact-final-main full CI, follow.
