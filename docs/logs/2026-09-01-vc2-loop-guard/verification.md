# VC-2 Loop detection: verification record

Date: 2026-09-01

## Commands and results

| Command | Result |
| --- | --- |
| `go build ./...` | exit 0 |
| `go vet ./internal/runtime/` | clean |
| `go test ./internal/runtime/ ./internal/rpc/ ./internal/storage/...` | All ok (runtime 103s, rpc 33s; exit 0) |
| `just ci` (full gate, including fmt-check / UI build / Playwright smoke) | exit 0 |

## New tests (`internal/runtime/loopdetect_test.go`)

- `TestServiceToolLoopDetected`: scripted model makes 8 same-parameter echo
  calls; the sixth repeat triggers → `run.failed`, `cause_category =
  loop_detected`, bounded message (no engine internals), exactly 1 terminal event,
  and 5 `tool.finished` entries remain in Journal (the triggering batch is
  discarded with the failure; see consume error-branch semantics).
- `TestServiceToolLoopWithinLimit`: 5 repeats + closing message → `run.completed`
  (legitimate repetition within the limit is unaffected).
- `TestLoopWindowCountsAndEvicts`: counts/eviction for window 10/limit 5
  (alternating signatures do not trigger; the 12th same signature triggers),
  different tool names produce different signatures, and tool-error results
  participate in the signature.
- Existing guard regressions: `TestServiceMaxToolTurnsBreached`,
  `TestServiceMaxToolTurnsWithinCap`, and
  `TestServiceBudgetCircuitBreakerStopsToolTree` all pass unchanged (the new
  guard stops at 6 repeats, before the default turn/budget limits, without
  changing their trigger paths).

## Contract synchronization

- `schemas/events/payloads/run.failed.json`: the cause_category enum adds
  `loop_detected` (wire contract).
- `docs/AGENT-VIVY-ARCHITECTURE-V0.md` ADR-004: the category list is completed
  (`human_timeout` was also missing and was corrected).
- No UI change: `ui/src/lib/failure.ts` directly displays the bounded server
  message for non-`provider_error` categories, so `loop_detected` works
  automatically.

## Smoke policy

The direct change is a kernel guard on a path not visible in the browser (the UI
reuses existing `run.failed` error-bar rendering); the real-model trigger needs a
provider key (no local mock after TEST-1). Manual trigger steps are in
acceptance.md, and automation is covered by scripted-model integration tests.
