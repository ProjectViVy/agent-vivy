# ND-0 acceptance

Goal (docs/plans/nudge/ND-0.md): prove the pinned Eino surfaces can support
the specified ordering and metadata without a second runtime. Scope:
integration evidence and minimal test-only helpers — no production code or
dependency upgrades. Met.

1. `internal/runtime/nudge_contract_test.go` exists and reuses
   `newTestService` conventions, `waitForRunStatus`, `replayAll`-style
   journal assertions via the decorating `contractJournal`, a real
   `adk.Runner` + scripted capture model + channel-controlled tools.
2. Correlation/order evidence delivered for both adapter flavors:
   `compose.GetToolCallID` at each adapter is the exact requested id; the
   barrier prevents inner model entry while a result's append is paused;
   the next input carries exactly one result per id. Baseline shows the
   engine may enter early without the barrier — proving the need for the
   barrier, not a failure.
3. The test-only WrapModel boundary waits on the state contract's
   completion surface; the wait does not prevent Eino from yielding tool
   results to the consumer; proven on both Generate and Stream.
4. Failure/shutdown evidence: recoverable metadata stays keyed to c1 and
   never leaks to c2 while Eino sees nil error; journal-append failure
   aborts the wrapper and the inner model count stays unchanged; context
   cancellation while waiting releases with no leaked wait; approval
   interrupt converts nothing and resume uses a fresh state; provider
   retry sees an identical prepared input with exactly one scheduling
   action; non-streamed tool requests journal before their results;
   compaction runs before the wrapper and its internal calls bypass it.
5. Commands run:
   `go test -timeout 20m ./internal/runtime -run '^TestNudgeContract' -count=1 -v` — 10/10 PASS.
   `go test -race -timeout 20m ./internal/runtime -run 'TestNudge|TestToolFailure' -count=1` — all pass except the enhanced-adapter subtest, which trips an upstream Eino race (compose/tool_node.go:1253); recorded as a finding.
   `just ci` — PASS (including the re-pinned `conformance_results.json` internal digest, which the new test file necessarily moved).
6. No deadlock found; ND-2's design assumptions are resolved or confirmed
   (see verification.md). One new contract requirement surfaced: the
   barrier must key on the input's trailing tool-call ids, and `Take` must
   be peek/idempotent per batch — both are captured for ND-2/ND-3.
7. No alternate loop was introduced; no production code changed. ND-0
   passing is not product acceptance — downstream Stories remain gated on
   their own evidence.
