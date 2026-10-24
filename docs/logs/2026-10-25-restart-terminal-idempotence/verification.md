# Verification — restart terminal idempotence

Host: macOS (darwin/arm64), Go pinned by `GOTOOLCHAIN=go1.26.4` (the ambient Homebrew
Go 1.27.1 breaks `golang.org/x/net v0.54.0`). Lane:
`.worktrees/restart-terminal-idempotent` on `fix/restart-terminal-idempotent`,
branched from `fix/tool-failure-continuation` @ `e858f878` (which carries the
tool-failure-resilience delivery plus the proxy dial-allow fix). The root `stable`
tree holds another lane's uncommitted work, so this lane did not merge into it.

## Run and green

```text
gofmt -l internal docs schemas                   # no output
GOTOOLCHAIN=go1.26.4 go build ./...              # clean (ui/dist copied into the worktree)
GOTOOLCHAIN=go1.26.4 go vet ./internal/runtime/ ./internal/storage/...
                                                 # clean
GOTOOLCHAIN=go1.26.4 go test -p 1 -count=1 ./internal/...
                                                 # no FAIL line in any package
GOTOOLCHAIN=go1.26.4 go test -p 1 -count=1 ./internal/runtime/
                                                 # ok 66.142s
GOTOOLCHAIN=go1.26.4 go test -p 1 -count=1 ./internal/storage/...
                                                 # storage 0.414s, migrations 3.313s,
                                                 # postgres 0.376s, sqlite 8.785s — all ok
GOTOOLCHAIN=go1.26.4 go test -p 1 -count=1 ./internal/runtime/ -run TestServiceRecover
                                                 # ok (all five recovery cases)
git diff --check                                 # clean
```

## The regression, and proof it pins the defect

`TestServiceRecoverReconcilesRowAfterClosedJournal`
(`internal/runtime/recovery_test.go`) builds the exact stuck shape the board
recorded: an `active` run row whose journal already holds a `run.failed` terminal
(appended through the real backend, so `Append` genuinely returns
`storage.ErrRunClosed` on the next attempt). It then restarts the service over the
same durable state and asserts the row becomes `failed`, the journal still holds
exactly one terminal, and a second `Recover` appends nothing and leaves the row
terminal.

Run against the pre-fix `service.go` (the change reverted, test unchanged):

```text
--- FAIL: TestServiceRecoverReconcilesRowAfterClosedJournal (30.07s)
    recovery_test.go:219: run run-stuck-terminal never reached status failed (last status active)
```

That is the boarded symptom: the row never leaves `active`, so every recovery pass
re-enumerates it. With the fix the same test passes.

Existing tests kept green without modification: `TestServiceRecoverResumableApproval`,
`TestServiceRecoverExpiredApprovalFails`, `TestServiceRecoverOrphanRunFails`,
`TestServiceRecoverNothingToDo`, the goal-recovery shutdown path, and the whole
SQLite/PostgreSQL/migrations conformance set (which exercises the new DSN on every
open).

## Observed flakes, with evidence they are load-related

Two `waitForRunStatus`-polling tests failed once each under full-suite load on this
16 GB host and passed on every isolated and repeat run:

- `TestINOFYWorkflowCancelPropagates` — failed once in a full `./internal/...` run on
  the **parent** tree `e858f878`, before any Lane 3 edit; passed 3× isolated
  (`-count=3`) and the package re-ran green (67.054s).
- `TestUnadmittedRunCannotReportCurrentGoalFromSameSession` — failed once in a
  combined `./internal/runtime/ ./internal/storage/...` run on this lane; passed 5×
  isolated (`-count=5`) and the package re-ran green (66.142s).

Neither test touches the reconciled path: both drive a healthy run to its terminal,
where `emitTerminal` behaves exactly as before (the `reconciled` branch is entered
only when `Append` returns `storage.ErrRunClosed`). `_busy_timeout` cannot affect
them either — each test opens its own temporary database and the backend holds a
single connection, so there is no second handle to wait for. Recorded here rather
than boarded; if the owner sees them again on an unloaded Linux gate they are worth a
separate look.

## Not run, with reasons

- `just wiring-ci`, `just kernel-ci`, `just d02-ci` — **unrun**. They require an
  independent Linux-filesystem checkout with its own pinned Laputa; macOS cannot
  certify them. Left to the owner.
- Real-path browser smoke at `http://127.0.0.1:3015` — **pending**. Reproducing the
  original `SQLITE_BUSY` window on demand needs a second write handle against a live
  `data/vivy.db`, which the air-gap rule (ST-2) puts out of bounds for an agent
  session; `acceptance.md` §5 writes the steps for the owner.
- PostgreSQL conformance ran only through the repository's existing
  `./internal/storage/postgres` package tests; a DSN-backed PostgreSQL instance was
  not exercised (no change here touches it).
- Windows native owner checks — out of scope for this lane.
