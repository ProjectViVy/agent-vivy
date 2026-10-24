# Provider tool-result-order rejection: one bounded retry

## What changed

A child run's first model call intermittently fails with an HTTP 400 whose body
carries `tool call result does not follow tool call (2013)`. Retrying the
identical payload by hand recovers, so the rejection is transient on the
provider side rather than a defect in the feed Vivy sends. Before this change
`internal/provider` and the runtime had no retry at all, and the run died with
`模型服务拒绝了本次请求（HTTP 400）`.

The recovery rides the existing Eino ADK seam instead of adding a second retry
mechanism:

- `Service.modelRetryDecision` is now the single decider bound into
  `Engine.SetRetryDecider`. It consults the overflow recovery decider first (the
  only one that rewrites the call input) and then the new tool-result-order
  decider.
- `isToolResultOrderFailure` classifies the fault by reusing `providerStatus`
  and the existing `providerStatusPattern`. There is no second status parser.
- `toolResultOrderRetryDecision` grants one retry of the identical payload per
  run, tracked in the new `Service.toolOrderRetried` map, which dies in
  `cleanupRunState`. Its budget is separate from `overflowRecovered` because the
  two diagnose different faults, but both draw from the same `modelMaxRetries`
  physical retry, so whichever grants first consumes it.
- The decision emits `auto_retry.started`, and the existing consume loop closes
  the pair with `auto_retry.finished`. The awaiting map it uses was renamed
  `overflowAwaiting` → `autoRetryAwaiting` because it is no longer
  overflow-specific.
- The full provider error text is logged at `slog.Error`. A masked payload
  ordering bug has to stay diagnosable, so the retry does not swallow it.

One correctness detail the ADK forces: `ShouldRetry` is consulted once more
after the final attempt, where a granted retry can never happen. Granting one
there would publish an `auto_retry.started` with no matching retry, so
`modelRetryDecision` declines any `RetryAttempt > modelMaxRetries`. That guard
also covers the overflow decider, whose one-per-run budget previously only
coincidentally matched the retry budget.

## Scope

- `internal/runtime/service.go`, `internal/runtime/engine.go`,
  `internal/runtime/overflow.go`.
- Tests: `internal/runtime/tool_result_order_test.go` (new),
  `internal/runtime/overflow_test.go` (the scripted model gained a per-call
  error override).
- No wire, storage, migration, UI, policy or provider change. `MaxRetries` stays
  at 1.

## Explicitly not done

- No retry, backoff or policy for `network_failed` tool errors; whether to retry
  those stays the model's decision.
- No request sampling to prove why the provider rejects the first child call.
  The retry makes the run survive it; the root cause on the provider side is
  still unconfirmed.
- No second retry attempt. A second 2013 fails honestly through the existing
  `providerHTTPFailure` classification as `internal_error`.
- No `not_configured` reason word and no capability probe.
