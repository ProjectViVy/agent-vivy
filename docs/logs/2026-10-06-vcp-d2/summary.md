# VCP-D2 — Overflow compact-and-retry recovery

## What landed

Provider context overflow (or silent `stop_reason=length` truncation with
empty output) now triggers **one** auto-compact + retry inside the live run;
a second overflow in the same run fails honestly. Mirrors pi's
`_runAutoCompaction("overflow")` one-shot semantics.

## Eino capability check (mandatory)

Inspected pinned `github.com/cloudwego/eino@v0.9.13` before writing code:

- `adk/retry_chatmodel.go` — `TypedRetryContext` (InputMessages /
  OutputMessage / Err / RetryAttempt) and `TypedRetryDecision` (Retry /
  ModifiedInputMessages / PersistModifiedInputMessages / RejectReason /
  Backoff / RewriteError). Documented for "context compression or message
  trimming" — exactly this seam.
- `adk/chatmodel.go` — `ChatModelAgentConfig.ModelRetryConfig` and the
  wrapper order: retry wrapper sits at position 4, outside the event
  sender (5) and user `WrapModel` middlewares (6), so a retried call
  re-traverses the full middleware chain (nudge, compaction, mounts).
- `adk/failover_chatmodel.go` — rejected: it swaps models, not context.
- Checkpoint seam — rejected: checkpoints exist only at interrupt
  boundaries, not mid-model-call, so resume-after-compact was impossible;
  `PersistModifiedInputMessages` persists the compacted input into run
  state instead, which is strictly stronger.

No checkpoint-resume path was viable for mid-call errors; `ModelRetryConfig`
is the Eino-native mechanism and covers the whole requirement, so no custom
loop machinery was added.

## Wiring

- `Engine` builds the agent with `ModelRetryConfig{MaxRetries:1}` and a
  nil-safe late-bound `ShouldRetry` closure; `Engine.SetRetryDecider` fills
  the slot after `NewService` exists (decider needs Messages, Compactions,
  the run's governance sink). Child engines (`restrictedView`) never get a
  decider — they fail fast as before.
- Decider (`overflowRetryDecision`) gates on
  `Compaction.Enabled` and classifies errors via `overflowPatterns`
  (adapted from pi `packages/ai/src/utils/overflow.ts`; `rate limit` /
  `too many requests` / `throttl*` excluded) or `isEmptyLengthStop`
  (pi case 3: `FinishReason=="length"` + empty content + no tool calls).
- `compactForOverflow` folds **pre-run history only** through the shared
  `foldToCompactionRecord` core (extracted from `CompactSession`; bypasses
  the busy gate since the run holds it, and the trigger-percent gate since
  the overflow IS the trigger). Pending-turn rows (`msg.RunID == runID`)
  stay verbatim — in-flight tool calls keep their `tool_calls` rows.
- Retry input = `[preamble, summary-user, kept tail, pending turn]`.
- Observability (journal + face): `auto_retry.started{attempt,reason}`,
  `context.compacted{mode:"overflow-recovery"}`, `provider.retry{reason}`
  (when eino surfaces WillRetryError), `auto_retry.finished{success}`
  emitted by the consume loop on the first outcome via `overflowAwaiting`
  — decider grants the attempt, consume closes the pair (`run` → attempt
  map, cleaned in `cleanupRunState`).
- New domain events `auto_retry.started` / `auto_retry.finished`;
  `provider.retry` payload gains `reason`; `GovernanceEvent` gains
  `Attempt`/`Success`. Vocabulary test bumped to 64 (also covers the two
  C3 events `session.cloned_from` / `session.imported` that landed without
  a count update).

## Notes for reviewers

- One-shot guard: `overflowRecovered[runID]` — set before compaction runs,
  so a retry that overflows again never loops compaction.
- `finish(false)` is emitted inside the decider on bail-out paths;
  `finished(true)` can only come from the consume loop because the decider
  cannot know the retried call's outcome.
- The late-bound decider means a nil slot (child engines, engines without
  a service) behaves exactly as before: every model error propagates.
