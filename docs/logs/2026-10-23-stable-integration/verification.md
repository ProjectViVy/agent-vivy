# Stable-push verification

Final executable source: `8b00475b1ea84a9013f77eadd9682328f3415976`.
The following documentation-only records do not change that source.
Independent Linux checkout:
`/home/mastwet/work/vivy-issue91-acceptance/agent-vivy`; Go1.26.4,
Node24.21.0, GOMAXPROCS2, GOFLAGS=-p=1, process-local direct GOPROXY.
Doctor confirms its own clean pinned Laputa; the source checkout is clean.

| Final check at8b00475b | Actual result |
| --- | --- |
| `just wiring-ci` | Pass:16 Node and111 executed Go tests, zero skips. |
| `just kernel-ci` | Pass:1970 executed owner tests, plus actual minimal pack/Inspect/boot test1. One existing optional `TestBackendGrepRipgrepHonorsGitignore` skip because Linux rg is unavailable. |
| `go vet -tags=vivy_headless ./internal/provider ./internal/app` | Exit0. |
| Full App, `go test -json -tags=vivy_headless -count=1 -timeout=35m ./internal/app` | 160 tests/subtests pass, zero failures. Two existing opt-ins skip: cognitive fixture capture and real-provider smoke. |
| `go test -race -json -tags=vivy_headless -count=1 -timeout=10m -run '^Test(JournalAppendPreservesCancelledTransactionCause\|CacheWarm)' ./internal/storage/sqlite ./internal/runtime` | 22 tests/subtests pass, zero failures/skips; no race reports. |
| Native changed-owner checks | Deterministic RED then GREEN; normal warming, mandatory storage failure, cancelled paid drainage and SQLite vet pass. See the [correction](../2026-10-23-sqlite-cancelled-admission/verification.md). |
| Real provider/tool benchmark | At397b6f17:12/12 pass,39 model calls,29 tools,104170 provider-reported tokens. See the [benchmark](../2026-10-23-live-tool-benchmark/verification.md). No new paid calls are attributed to8b00475b. |
| Broader platform gates | Retain their actual D14 source revisions, including native full `just ci` at5b0b572d. See [D14 verification](../2026-10-22-issue91-d14-acceptance/verification.md). These are not final-source reruns. |

The initial c02c4b30 kernel attempt failed the streaming warmer's completion
case after a cancelled append returned only sql.ErrTxDone. Twenty unchanged
focused passes did not count as resolution. The fix preserves both native
causes for that specific cancelled transaction. Its deterministic test uses
actual SQLite SQL/rollback and driver-boundary fault injection; it does not
claim to recreate watcher timing. Independent source review accepts the fix.
Original and final raw Linux outputs remain under
`.workspace/verification/stable-push-2026-10-23/`.

The102 pre-fix outgoing commits were scanned for common key/private-key
literal shapes. Seven file/commit matches were reviewed as explicit redaction
fixtures/canaries in three test files; no live credential was identified.
The new correction/record files contain no credential-shaped literal. All
outgoing author/committer metadata is human mastwet; `gh api user` confirms
that push account. `git diff --check` passes for the integration delta.

Before integration, local stable was clean at cd0d4d3f; freshly fetched remote
stable was fde3c1f, with no divergent remote commit. The GitHub branch read
reports protected=false. Integration uses `git merge --ff-only`, normal
`git push origin stable`, and remote/local HEAD comparison afterward. There
is no force push, protection bypass, release creation or database operation.

Linux full-race, PostgreSQL, Docker, full native CI and browser gates are not
repeated for this provider/SQLite delta. Their unchanged-owner evidence retains
its historical source; the affected native/Linux checks above cover this
integration scope. The human Tiny Face checklist and tracked baseline/CI intake
remain open. Hosted CI is reported separately from local gate results.
