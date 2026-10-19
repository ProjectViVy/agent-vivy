# S1: Face owner lifecycle and shutdown race

## Scope and isolation

- Story: `docs/superpowers/plans/ci-closure/S1.md` in the coordinator lane; requirement R1 of the CI closure design.
- Baseline: `e46d332362608062b44509810628e66bce97a712`.
- Repair checkout: `/home/ubuntu/memory-write-model/agent-vivy/.worktrees/ci-repair-face/agent-vivy`, created with `node /home/ubuntu/memory-write-model/agent-vivy/scripts/lane-new.mjs ci-repair-face --base e46d332362608062b44509810628e66bce97a712` from the original repository directory. The command succeeded and provisioned the lane's private sibling Laputa.
- Only the two owned test files and this evidence file changed. No production code, owner guard, unknown-commit guard, dependency pin, or sibling Laputa file was edited. No delivery commit, push, PR, merge, deployment, Git configuration edit, or remote edit was made.
- The already built coordinator `ui/dist` was copied into this lane's `ui/dist` solely for Go embed compilation. UI was not rebuilt; no shared mutable `node_modules` was used.
- Test configurations use temporary roots. No production `data/`, tenant journals, or `~/.vivy` were used.

All Go commands below ran from the repair checkout on Linux with these process-local environment variables (also used for lane creation):

```sh
GIT_CONFIG_GLOBAL=/dev/null
GIT_AUTHOR_NAME=大湿
GIT_AUTHOR_EMAIL=1136523089@qq.com
GIT_COMMITTER_NAME=大湿
GIT_COMMITTER_EMAIL=1136523089@qq.com
GOTOOLCHAIN=go1.26.4
GOPROXY=https://proxy.golang.org,direct
```

## Red: observed baseline failures

Before editing either test file:

```sh
go test -count=3 ./internal/app -run '^TestSelectedFaceUsesOneApp$'
```

Exit 1, all three executions failed at baseline `facehost_selected_test.go:101`:

```text
compose headless app: app: open memory service: memory: active Home belongs to another data root or core lease
FAIL agent-vivy/internal/app 0.732s
```

The first App is registered only for end-of-test cleanup. The same test then composes another App while the process-wide memory Home still belongs to the first App's data root/core lease. The rejection is correct; the fixture ownership lifetime is wrong. `App.Close` already waits for teardown and is idempotent, so changing the guard is unnecessary.

```sh
go test -race -count=10 ./internal/faceprocess
```

Exit 1, race detector reported:

```text
WARNING: DATA RACE
Write: stubSelectedFace.func2(), selected_test.go:49
Previous read: Run.func1(), launch.go:112
--- FAIL: TestFaceLaunchShutdownDeadline
    --- FAIL: budget_expiry_reports_incomplete_cleanup
        race detected during execution of test
FAIL agent-vivy/internal/faceprocess 0.796s
```

The deadline subtest slept before cancellation, released its blocked provider on return, and immediately restored global `runSelectedFace` in test cleanup. A deadline return deliberately does not join the provider goroutine. Releasing a channel alone did not establish a completion edge before restoring the global override.

## Minimal repair

- `internal/app/facehost_selected_test.go`: explicitly close the selected-face App, checking the close error, before composing the headless App. Keep the idempotent cleanup registered for early assertion failures. All selected-face and headless assertions remain.
- `internal/faceprocess/selected_test.go`: the blocking provider signals that it started, cancels the launch context itself, and signals body completion with a deferred channel close. Deferred test teardown releases the provider and waits for that completion before test cleanup restores globals. The startup-failure branch does not wait for a provider that was never invoked. Remove the sleep and its cancellation goroutine; keep the 50 ms teardown budget and shutdown-error assertion, and additionally assert the failed result status.
- Existing graceful shutdown and shared-deadline tests remain intact. No sleeps, skips, xfails, deadline relaxation, discarded stderr causes, or live-network regressions were added. The existing failing tests supplied red coverage; no production change or new test helper was needed.

## Green: executed checks

| Command | Result |
| --- | --- |
| `go test -count=3 ./internal/app -run '^TestSelectedFaceUsesOneApp$'` | Exit 0, `ok agent-vivy/internal/app 0.821s` after the App fixture fix. |
| `go test -race -count=10 ./internal/faceprocess` | Exit 0, `ok agent-vivy/internal/faceprocess 1.758s` on the final fixture code. Includes graceful stdin-unblock, shared teardown deadline, and budget-expiry cases. |
| `go vet ./internal/faceprocess ./internal/app` | Exit 0, no diagnostics. |
| `go test -race -count=3 ./internal/app -run '^(TestSelectedFaceUsesOneApp\|TestIssue58AppOwnedMemoryLifecycle\|TestIssue58AppCompetingMemoryConstructors\|TestIssue58AppRunReleasesOwnedMemory)$'` | Exit 0, `ok agent-vivy/internal/app 37.589s`. Independent checks retain rejection of duplicate/foreign owners, isolation of failed constructors, stale-close protection, and release/reopen. |
| `go test -race -count=3 ./internal/app -run '^TestIssue58CompactionCrashCutsHostReopen$'` | Exit 0, `ok agent-vivy/internal/app 5.313s`. Existing unknown-commit/pending-intent guard and reopen recovery remain intact. |
| `go test -race -count=3 ./internal/modules/memory -run '^TestIssue58(MemoryOwnedRootAndRelease\|BoundProviderCannotSubstituteForeignActive\|MemoryConcurrentOwnedAcquireRelease)$'` | Exit 0, `ok agent-vivy/internal/modules/memory 1.217s`. |
| `gofmt -w internal/app/facehost_selected_test.go internal/faceprocess/selected_test.go` | Exit 0. |
| `git diff --check` | Exit 0. |

The backslashes before table-cell pipes above are Markdown escapes, not part of the executed shell regexes.

## Acceptance and untested boundaries

S1's focused checks pass on Linux. The selected-face fixture now relinquishes its owner before the next App is composed; the deadline fixture now drains its provider before restoring the global launch seam. Production owner protections and deadline semantics are unchanged.

This is not native Windows acceptance. Full `just ci`, the complete App suite, aggregate repository race checks, UI/build acceptance, compatibility matrices, real PostgreSQL, and a second independent integration pass were not run by this worker; those remain the S6 integration boundary. No claim is made for SDK/Toolkit/BML script repairs or the Laputa cancellation/pin update owned by other Stories.

## Patch handoff

Export target: `/home/ubuntu/memory-write-model/ci-closure-results/face.patch`.

```sh
git add -N -- docs/logs/2026-10-19-ci-closure/face.md
git diff --check
git diff --binary -- internal/app/facehost_selected_test.go internal/faceprocess/selected_test.go docs/logs/2026-10-19-ci-closure/face.md > /home/ubuntu/memory-write-model/ci-closure-results/face.patch
```

The log is intentionally the single Story evidence file requested by the owner. Integration owns publication and aggregate acceptance; this handoff remains uncommitted.
