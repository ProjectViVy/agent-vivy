# Acceptance

1. With an active Goal run, human `plan/enter` first yields a persisted `goal.paused` event and a disarmed Work view. Plan remains inactive until that run reaches terminal cleanup; then `plan.entered` follows in the same session work stream.
2. If the Plan entry request is cancelled before the run drains, Plan remains inactive. A new Plan entry request also waits for the still-live run. Deleting the session during the drain returns an error to the Plan caller and leaves no session or work state behind.
3. Reviewing a submitted Plan with `execute_once` resumes the exact origin call and leaves Goal absent. `start_goal` stores one Goal, and its first round is admitted only after the origin run completes. Repeating the same decision returns the same Journal event without rearming a dormant Goal; an unfinished paused Goal produces an explicit conflict without replacement.

The scenario tests use SQLite session Work/Journal records and the real Service/RPC transition path. Live PostgreSQL acceptance is outstanding.
