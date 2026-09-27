# PG-4 Plan and Goal handoff

Human Plan entry now pauses an active Goal durably, cancels its owned run, waits for terminal cleanup without holding admission or projection locks, and revalidates the transition token and WorkVersion before entering Plan. A failed drain leaves Goal paused and Plan ineffective. A later request against the paused Goal must still drain its old run. Session deletion invalidates the transition before it can publish an effective Plan.

Plan review decisions continue through the existing Eino checkpoint and `Service.DecidePlan` path. `execute_once` does not create a Goal. `start_goal` creates one Goal in the atomic decision event; the existing origin-run terminal wake admits its first round only after cleanup. Identical decisions replay the stored result without rearming a dormant Goal, and an unfinished paused Goal blocks replacement.

Scope: Vivy runtime and RPC coordination. No Studio, UI, new runner, Journal, policy path, provider upgrade, or database schema change was made. Docker and live PostgreSQL verification remain deferred at the user's direction.

Review correction: Plan now revalidates the paused WorkVersion before claiming cancellation. A human `goal/resume` uses one Service boundary for its durable mutation and Goal rearm, ordered against that claim. If resume wins, the stale Plan does not cancel the run or enter Plan; if Plan claims cancellation first, resume conflicts while the run drains. The phase boundary is shared by public `EnterPlan` and the deterministic regression test. Real authenticated RPC coverage also verifies a connection-bound Plan request, cross-session rejection, and the initial `start_goal` RPC wake.
