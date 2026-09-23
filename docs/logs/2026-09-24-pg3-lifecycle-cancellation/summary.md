# PG-3 Task 2: Lifecycle and cancellation

This iteration covers Goal reservation invalidation, old-revision reporting, suspended approval/question admission, cancellation, round exhaustion, and recovery after a committed GoalRun. The scenarios use real SQLite work, run, message, and Journal records through the existing Service. The only behavior correction makes RPC WorkView derive disarmed activation and an empty current RunID from a durably paused, blocked, completed, or cleared Goal while its old process-local run is still cancelling.

The durable work mutation remains authoritative and commits before `CancelGoal`; the admission path re-reads current work and its atomic `CommitGoalRun` checks the expected version and GoalRef. Terminal cleanup settles the admitted run with its actual RunID, and the existing per-run BudgetLedger is unchanged. A simulated crash after the atomic GoalRun commit is recovered fail-closed without refunding or replaying the round.

Pinned Eino v0.9.13 already provides ADK Runner `Run`/`ResumeWithParams`, `components/tool.StatefulInterrupt`, and `compose.GetToolCallID`. Vivy's existing runtime adapter and checkpoint path handle approval/question suspension and resume. No Eino pins, orchestration loop, schema, dependency, queue, or new source of truth changed. Task 1's human-intent and duplicate-wake implementation was reused. The unrelated modified generated UI files were not included.

The checked-in SDK conformance source digest was refreshed for the changed `internal/` tree. This delivery does not claim the later PG-3 browser/model walkthrough or live PostgreSQL acceptance.
