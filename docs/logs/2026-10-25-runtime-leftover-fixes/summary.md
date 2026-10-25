# Runtime leftovers from the 2026-10-25 changes

## What changed

The audit of the 2026-10-25 deliveries found five superseded or
never-implemented pieces. Each is fixed or deleted here; no new retry
mechanism, runtime or configuration knob was added.

1. **Engine reload keeps the retry decider** (`internal/runtime/service.go`,
   `internal/runtime/compaction_service.go`). `applyPendingEngineReload`
   published a freshly rebuilt engine without calling `SetRetryDecider`, so any
   settings save that rebuilt the engine silently disabled overflow, bare-2013
   and tool-result-order recovery until restart. `NewService` and
   `applyPendingEngineReload` now publish through `Service.adoptEngine`, which
   binds the decider before the swap.
2. **One run-scoped model retry budget** (`internal/runtime/service.go`,
   `internal/runtime/overflow.go`). `overflowRecovered` and `modelRetries` were
   independent maps, so a run could compact-and-retry once from overflow and
   again from a 2013 in a later model call. That contradicted the documented
   contract ("only one retry per run across all reasons").
   `reserveModelRetry` now owns the single claim (fingerprint, awaiting marker,
   `auto_retry.started`); `grantModelRetry` and `overflowRetryDecision` both use
   it, and `overflowRecovered` is deleted.
3. **Compaction folds exclude child transcripts**
   (`internal/runtime/message_projector.go`,
   `internal/runtime/compaction_service.go`, `internal/runtime/overflow.go`).
   `CompactSession` and `compactForOverflow` listed every stored row, so a
   delegated child's transcript could become durable session history through
   the summary. Both now fold `Service.parentFeed`, the same child-filtered feed
   the run and `ContextStatus` use, and pass the filtered slice as the stored
   set so `afterTokens` cannot count child rows again.
4. **One overflow classifier** (`internal/runtime/service.go`,
   `internal/runtime/overflow.go`). `deterministic2013Phrases` duplicated
   `overflowPatterns` and missed forms such as `prompt is too long (2013)`, so
   such a rejection was replayed identically. `isBareProvider2013` now consults
   `isContextOverflow`; only the non-overflow payload phrase (`invalid function
   arguments json string`) remains a local literal. Three patterns were
   broadened (`maximum context length`, `reduce the length`, `context window
   exceeds`) so every previously excluded phrase still classifies.
5. **Dead request describer removed** (`internal/runtime/request.go`).
   `describeModelRequest` was orphaned when the wire event moved to
   `describeModelRequestV5`; its two test callers now use the live envelope.

Behavior change to note: a bare 2013 whose text names a context overflow is now
classified as overflow. A primary run compacts and retries with the rewritten
input; a child fails honestly because child compaction stays disabled.

## Tests

- `TestScheduleEngineReload` asserts the retry decider survives both an
  immediate and a deferred reload, including on a child view.
- `TestOverflowRetryConsumesBudgetBeforeToolResultOrder` now places the
  tool-result-order rejection in a separate model call at `RetryAttempt=1`, so
  it can only be declined by the run-scoped budget.
- `TestChildModelRetrySkipsOverflowRecovery` reads the unified budget map.
- `TestCompactSessionExcludesChildTranscript` and
  `TestCompactForOverflowExcludesChildTranscript` assert two folded rows and a
  marker-free summarizer input.
- `TestIsContextOverflowClassifier` and `TestBare2013NamedVariantDoesNotRetry`
  cover the broadened phrases.

## Scope

`internal/runtime` only. No protocol, storage, migration, provider-wire, policy,
UI or configuration change. `modelMaxRetries` stays 1, and the ADK retry seam is
used unchanged.

## Explicitly not done

- No compaction inside child runs; `CHILD-CONTEXT-COMPACTION` in
  `docs/TODO.md` §0.1 still owns that decision.
- No change to `max_tool_result_bytes` or the per-call `RetryAttempt` guard.
- No new retry reason, knob or backoff.
