# Verification: tool-result-order retry

Host: macOS (arm64), Go pinned to `GOTOOLCHAIN=go1.26.4`. Every command below
ran from the `fix/tool-result-order-retry` worktree at the candidate.

## Targeted tests

```text
GOTOOLCHAIN=go1.26.4 go test -p 1 -count=1 ./internal/runtime/ -run 'ToolResultOrder|Overflow'
ok  agent-vivy/internal/runtime  2.265s

GOTOOLCHAIN=go1.26.4 go test -p 1 -count=1 ./internal/runtime/ -run 'Overflow|Retry|Provider'
ok  agent-vivy/internal/runtime  2.739s
```

New cases in `internal/runtime/tool_result_order_test.go`:

| Test | Pins |
|---|---|
| `TestToolResultOrderRetryRecoversRun` | one 400 (2013) → identical payload replayed → run completes; one `auto_retry.started` with reason `tool_result_order`, one `auto_retry.finished` `success: true`, zero `context.compacted` |
| `TestToolResultOrderSecondFailureEndsRun` | two 400 (2013) → run fails; exactly one `auto_retry.started` (no loop), `auto_retry.finished` `success: false`, `run.failed` terminal present |
| `TestOverflowRetryConsumesBudgetBeforeToolResultOrder` | overflow takes the single physical retry, then a 2013 on the retried call publishes no second `auto_retry.started` |
| `TestIsToolResultOrderFailureClassifier` | 400 + `(2013)` only; a plain 400, a 500 carrying the same body, an overflow message, a transport error and `nil` all decline |

The error fixture is the observed provider shape already pinned by
`internal/runtime/service_test.go:2301`, not an invented one.

## The tests pin the defect

Reverting only the decider binding (`eng.SetRetryDecider(svc.modelRetryDecision)`
→ `svc.overflowRetryDecision`) and re-running:

```text
--- FAIL: TestToolResultOrderRetryRecoversRun (10.07s)
    tool_result_order_test.go:27: timed out waiting for run completes after retry
--- FAIL: TestToolResultOrderSecondFailureEndsRun (0.10s)
    tool_result_order_test.go:76: auto_retry.started count = 0, want exactly 1 (no retry loop)
```

with the backend logging `run failed ... body: {"error":"tool call result does
not follow tool call (2013)"}`. The binding was restored afterwards and the
suite re-ran green.

`TestOverflowRetryConsumesBudgetBeforeToolResultOrder` passes either way: it is
a guard on the shared retry budget, not a regression test for the binding.

## Full suite and static gates

```text
GOTOOLCHAIN=go1.26.4 go test -p 1 -count=1 ./internal/...   # exit 0, no failures
GOTOOLCHAIN=go1.26.4 go vet ./...                           # clean
GOTOOLCHAIN=go1.26.4 go build ./...                         # clean
gofmt -l internal schemas docs                              # empty
git diff --check                                            # clean
```

No flake was observed in this lane; every package passed on the first run.

## Not run on this host

- `just wiring-ci`, `just kernel-ci`, `just d02-ci`: these require an
  independent Linux/WSL checkout with its own pinned Laputa. macOS cannot
  certify them, so they are recorded as **unrun** and belong to the owner.
- The browser smoke at `http://127.0.0.1:3015` in `acceptance.md`: the fault is
  an intermittent provider rejection, so it cannot be provoked on demand. Items
  1–3 are therefore covered by the scripted-model tests above rather than by a
  live run, and stay **pending** for the next time the rejection reproduces.
  Items 4–6 are journal-shape assertions the tests already make.
- Full-product `just ci`: a separate platform/release gate, not re-run per lane.
