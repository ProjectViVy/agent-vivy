# Acceptance — TUI-STREAM-N5

## How a person can confirm it works

1. Normal-session streaming output, approval, and cancellation behavior is unchanged (regression surface; all TUI tests + just ci).
2. For a completed run's `run/subscribe` with a sufficiently large `after_seq`, the subscription ends immediately (the server-side subscriptions map count returns to zero), with no hanging connection.
3. For a nonexistent run id's `run/subscribe`, the subscription ends immediately, with no hang.
4. There is no user-visible UI change—this slice tightens protocol/resource cleanup and is purely defensive.

## Regression risks

- Early subscription termination appears to the client as a silent end of the
  notification stream; the durable TUI surface is driven by terminal events
  such as `run.completed`, does not depend on keeping the connection open, and
  neither Live implementation resubscribes after the terminal state.
