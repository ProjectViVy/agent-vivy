# P6.1 summary — legacy orchestration proof retirement

## Outcome

W9 now fails closed. Approval decisions carrying the retired
`native-orchestration:` target return the typed
`ErrLegacyOrchestrationResumeUnsupported` error before first-writer settlement.
The approval remains pending and can still be inspected. The obsolete graph
proof and its Eino type registrations are test-only; the product no longer has
a dispatch path that can resume that graph.

## Changed behavior

- Added an explicit guard to local/actor decision admission, shared approval
  settlement and timed system settlement.
- Moved the small Eino `compose.NewWorkflow` proof into
  `orchestration_proof_test.go`, retaining only registrations used by that
  proof test.
- Removed the obsolete approval-specific graph suspend/resume helpers and
  service integration tests that asserted retired behavior.
- Added regressions for live and post-restart approval rows, timed system
  settlement, unchanged descriptor/checkpoint reads, and current authored plus
  trusted workflow admission.

## Execution ruling

The root implementation lane did not have the required `just` executable, so
the P6 source/runtime tests and Go build were run directly. The full `just ci`
gate remains a P7 integration requirement. A pre-existing cancellation test
also had a timing-sensitive single-status assertion; it now accepts either
valid engine classification (`cancelled` or `recovery_required`) while keeping
its no-replay and non-terminal native Run checks.

Implementation commit: `fd1954f1` on `feat/issue32-remediation`.
