# Verification

Initial source: `c02c4b30`. Native venue is the independent
`.workspace/issue91-native/agent-vivy` checkout. Linux venue is the existing
`/home/mastwet/work/vivy-issue91-acceptance/agent-vivy`, with its own clean
Laputa `9acc999301d5d575da2cea902a1c6d213763f381`, Go1.26.4 and Node24.21.0.

| Check | Actual result |
| --- | --- |
| Initial `just wiring-ci` at c02c4b30 | 16 Node +111 Go tests pass; zero skips. |
| Initial `just kernel-ci` at c02c4b30 | FAIL: 1968 pass, one optional rg-dependent skip, `TestCacheWarmStreamingFiresAfterSettle` fails because the Run failed after a `model.request` commit returned only `sql.ErrTxDone`. Artifact stage not reached. |
| Original warming fixture, count20, unchanged source | 20 pass; zero skips/failures. This does not close the original failure by retry. |
| New SQLite regression before the fix | Deterministic RED, 0.421s: cancelled transaction lost `context.Canceled`; the other two cases pass. |
| Same regression after the fix | GREEN, 0.609s; three cases exercise real Append SQL/rollback, error identity and append/reopen continuity. |
| Native `go test ./internal/runtime -run '^TestCacheWarm(StreamingFiresAfterSettle\|SettlementFailureFailsOwningRun\|WaitIdleDrainsCancelledPaidCall)$' -count=1` | Pass, 2.594s. Real persistence failures still fail the owning Run; paid cancellation still drains. |
| Native `go vet ./internal/storage/sqlite` | Exit0. |
| `gofmt` and `git diff --check` | Pass. |
| Independent two-file source review | ACCEPT, no blocker; no reviewer test execution claimed. |

Native commands use the installed Windows Go toolchain. The first restricted
attempt could not access its existing build cache and did not run tests; the
formally approved invocation produced the RED/GREEN results above. An initial
inline WSL command stopped before tests; the explicit script-file invocation
then synchronized and verified the clean candidate. No gate was weakened.

The new storage fixture injects `sql.ErrTxDone` after real driver rollback;
it does not claim to recreate the database/sql watcher interleaving. Source
inspection of Go's `Tx.awaitDone`, `Tx.rollback` and `Tx.Commit` explains that
interleaving. The observed warming failure and unchanged count20 result remain
recorded at their original source. Final gate repetition follows this fix.

Eino capability check: no LLM capability is added. The inspected existing
`model.BaseModel.Generate`, `schema.StreamReader` and `adk.NewTypedRunner`
continue to own provider execution and orchestration. The gap is the SQLite
owner's lost native Go context cause; an Eino adapter cannot restore that
cause after storage drops it. No SQL identity leaks into runtime code.

All databases are temporary test files. Production Journals and credentials
are untouched. Raw Linux outputs remain under
`.workspace/verification/stable-push-2026-10-23/` in the Linux checkout.

## Final integration verification

At clean source8b00475b, independent Linux `just wiring-ci` passes16 Node
and111 Go tests. `just kernel-ci` passes1970 owner tests and the actual
minimal pack/Inspect/boot case, with one unchanged optional rg-dependent skip.
Provider/App vet exits0. The complete App suite passes160 tests/subtests with
two existing opt-in skips. Targeted SQLite/warming race passes22 with zero
failures/skips. See the [stable integration record](../2026-10-23-stable-integration/verification.md).
