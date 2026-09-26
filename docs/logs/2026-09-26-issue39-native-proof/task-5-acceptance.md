# Task 5 acceptance

## Human-observable verdict

Task 5 remains **NO-GO**. Run:

```text
'/mnt/c/Program Files/Go/bin/go.exe' test ./internal/runtime -run '^TestGraphConformanceBrokerReplayRisk$' -count=1 -v
```

The expected intentional non-zero result names
`TestGraphConformanceBrokerReplayRisk` and says that two same-process direct
calls to `ExecuteBrokerTool` re-invoked its in-memory fixture counter. The
probe uses the same run and input in one process. It does not create or restart
a Service, Engine, checkpoint, or fresh store; it injects no crash and measures
no real external side effect. Its bounded observation is only that repeated
direct calls through this broker API re-invoke the fixture, while the API seam
has no durable operation/result identity.

This is not G0 success and is not an injected crash/restart test. G0 remains
conservatively NO-GO because the Service/Eino/checkpoint/recovery criteria and
replay safety across a fresh store remain unproven. ORCH-02–08 remain blocked;
no user-visible behavior changed.

## Restart condition

G0 may be reconsidered only after approved scope supplies a Service-owned
durable effect operation contract and proves the original criteria together,
including an injected crash/restart through a fresh Service/Engine/store,
defined unknown-effect handling, and SQLite/PostgreSQL conformance. This
Task 5 probe is insufficient for that decision.
