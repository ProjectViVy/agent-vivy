# Verification

Worktree `/workspace/agent-vivy-qq`, branch `fix/qq-redial-test-boundary`, starts
from root `cc024efc2329a130eb93c35af88f94800357b1b5`. Commands run from
`plugins/qq` with `PATH=/workspace/agent-vivy/work/bin:$PATH` (Go 1.26.4).

## Failure reproduction

The parent's integrated provider-conformance gate reported the second attempt
as `connect 1, identify 0, resume 0` while the test demanded Resume already ran.
An unchanged local `go test -run '^TestRedialResumeAndGiveUp$' -count=20` passed
(6.948s), confirming this is scheduling dependent rather than a guaranteed failure.

A temporary test-only use of the existing `wsFactorySpy.onBuild` hook paused the
second factory invocation after it published the client and before returning it
to the supervisor. With the original wait/assertion unchanged,
`go test -run '^TestRedialResumeAndGiveUp$' -count=1` failed deterministically
(0.030s): `connect 0, identify 0, resume 0`, demonstrating the same premature
boundary. The temporary hook and scratch copy were removed before delivery.
Production code was never modified.

## Corrected fixture

- `go test -run '^TestRedialResumeAndGiveUp$' -count=20`: passed (0.938s).
- `go test -race -run '^TestRedialResumeAndGiveUp$' -count=20`: passed (1.988s).
- `go test ./... -count=1`: complete QQ plugin suite passed (0.407s).
- `go vet ./...`: passed.
- `gofmt` applied; `git diff --check` passed.

The existing timeout guards remain failure guards, not delays used to make the
test pass. Waiting for `done` establishes that terminal reconnect processing
actually ended and proves no further attempts can occur.

The parent explicitly owns source-hash evidence refresh, executed conformance
reproduction and final integrated `just ci` after this narrow fixture commit.
This lane does not claim those integrated gates passed or modify pass flags.

Integrated sealing also updates the QQ descriptor/YAML source declarations and conformance fixture to the canonical fixture-adjusted digest. Tree normalization of the declared digest confirms `ede386c84e08e19c43b161a535e8a1b70cf7eef98dafe1b725f1e7025c19c7ed`; publication flags remain unchanged and are independently executed by the root conformance gate.
