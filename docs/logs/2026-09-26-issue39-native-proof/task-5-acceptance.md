# Task 5 acceptance

## Human-observable verdict

Task 5 is accepted only as **NO-GO**. Run:

```text
'/mnt/c/Program Files/Go/bin/go.exe' test ./internal/runtime -run 'TestOrchestrationNative|GraphConformance' -count=1 -v
```

The expected evidence is a non-zero exit with both named cases. The
`TestOrchestrationNative` case reaches the real Eino Workflow/Service seam and
returns `ErrNativeOrchestrationUnimplemented`; the
`TestGraphConformanceBrokerReplayRisk` case reports two executions for the
same replayed broker operation. A zero exit, skipped/no-tests output, fake
lambda, or a claim that the other five observations compensate for missing
crash-safe effect identity would contradict this verdict.

The canonical Issue #39 index now records ORCH-01 as G0 NO-GO and leaves
ORCH-02–08 blocked. No user-visible behavior changed.

## Restart condition

G0 may be reconsidered only after the approved scope supplies a Service-owned
durable effect operation contract that can distinguish and safely resolve both
sides of the crash window. It must use stable effect identity, persist outcome
or broker idempotency across a fresh Service/Engine/store, define unknown
effect handling, and pass SQLite/PostgreSQL conformance before the original six
G0 criteria are rerun together.
