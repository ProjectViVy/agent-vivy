# Verification

Task environment and generated diagnostic DIVA overlay; actual ONNX/Garden/Mentle/Laputa composition with loopback model wire. The overlay uses official SDK compile/generate/seal APIs for diagnostic construction, not final source/artifact attestation.

- Actual activity/Restart/completed archive/live cancellation archive plus existing Work reconciliation: chain-activity-app-restart-work-second.jsonl, 4 pass/zero fail/skip, observed exit0.
- Real stale Work patch after native capture: chain-activity-app-stale-work-first.jsonl, 1 pass/zero fail/skip, exit0.
- go test -race -json ./internal/runtime -run '^(TestCognitive.*|TestMemoryLoopCapture.*|TestDeleteSession.*)$' -count=1: chain-activity-runtime-final-race.jsonl, 44 pass/zero skip/exit0.
- go test -race -json ./internal/observerhost -count=1: chain-activity-observer-barrier-green.jsonl, 12 pass/zero skip/exit0.
- Default App/cognitive/default catalog regression: chain-activity-default-app-modules.jsonl, 163 pass/28 conditional skips/exit0 (before adding the separately passing stale-Work App probe). Default generation's DIVA omissions are not positive acceptance.

Preserve baseline S05 empty Pulse/Recap, missing native/API compile failures, no-new-delivery timeout, archive-head/live deletion failures, native capsule capacity failures, runtime watermark high7 versus real source2, and concurrent cursor barrier 31 failures/32 deliveries. Preserve intermediate test fixture mistakes: guessed helper/path names, unauthorized post-delete read, global-head equality, empty-source nil/slice equivalence, archive chunk aggregation and Restart Session rehydration. These never count as production fixes or successful full acceptance.

Final combined App regression: chain-activity-app-final-prefix.jsonl, 15 pass/zero fail/skip, observed exit0. Includes activity/Restart/archive/stale Work, automatic reflection, no-change/policy, capture terminal matrix and replay/conflict. The preceding combined run (chain-activity-app-final-all.jsonl) had 14 pass/one readiness failure; retain it. Await the actual native source prefix after canonical input, never a fixed delay. git diff --check passes; required full phase gates remain pending.
