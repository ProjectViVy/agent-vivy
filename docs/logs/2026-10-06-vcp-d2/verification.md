# VCP-D2 — Verification

## Commands

- `go build ./internal/...` — clean.
- `go vet ./internal/runtime` — clean.
- `gofmt` applied to all touched files.
- `go test ./internal/runtime -run 'Overflow|Compact' -count=1` — PASS
  (0.819s).
- `go test ./internal/runtime ./internal/domain -count=1` — PASS
  (46.9s runtime; domain vocabulary test updated to 64).
- `go test ./sdk/internal/conformance/ -count=1` — PASS after re-pinning
  the internal source digest
  (`a74b626a…` → `c3986349276ad0dfe40297f6c177e4736e0a09f66c7d18f49bb96a1d501f8f38`).

## New tests (`internal/runtime/overflow_test.go`)

- `TestOverflowCompactionRetryRecoversRun` — scripted overflow on call 2 of
  a 2-run session: run completes; retried call's input contains the
  compaction summary user message and the pending user turn verbatim;
  journal carries `auto_retry.started`, `context.compacted
  {mode:overflow-recovery}`, `auto_retry.finished{success:true}` (exactly
  one each); `provider.retry` at most once with reason `context_overflow`.
- `TestOverflowSecondFailureEndsRun` — the retry itself overflows: run
  terminal `run.failed`, exactly one `context.compacted` (no compaction
  loop), `auto_retry.finished{success:false}`.
- `TestOverflowLengthStopTriggersRecovery` — pi case 3: empty
  `FinishReason="length"` response triggers the same recovery path; run
  completes.
- `TestOverflowDisabledPolicyAcceptsError` — no `Compaction` policy →
  decider declines, run fails without any compaction.
- `TestIsContextOverflowClassifier` — pattern table incl. rate-limit /
  throttling exclusions and joined error chains.
- `TestIsEmptyLengthStop` — detects only empty truncated outputs.

## Evidence notes

- A debug run journal (captured during development) showed the real event
  sequence: `model.call.finished{status:failed}` → `auto_retry.started` →
  `context.compacted` → `model.request` (compacted input) → `model.delta` →
  `run.completed` — 3 model calls total.
- `WillRetryError` was verified to be consumed inside eino's retry
  machinery in this version and does NOT reach the mapper as an AgentEvent;
  `auto_retry.finished` therefore hangs off the service-level
  `overflowAwaiting` map, not the event. `provider.retry{reason}` remains
  wired on both WillRetryError branches for paths that do surface it.

## Deferred / out of scope

- `just ci` deferred to H1 per story convention.
- Child (task/session) engines get no decider — known limitation, they
  fail fast on overflow.
