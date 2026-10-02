# Verification

- `go test ./internal/runtime ./internal/rpc` — pass. New tests:
  `TestTrajectoryMultipleCallsStableIdentity` (separate rows per call,
  latest usage replaces own attempt, stable IDs on rebuild),
  `TestTrajectoryActiveWaitCancelAndInterrupted` (active ≠ failure,
  waiting on run_activity, cancellation interrupts open calls, nil vs
  zero usage), `TestTrajectoryWatermarkWindow` (prefix watermarks,
  has_older_runs, window clamp).
- `pnpm typecheck`, `pnpm test` (527 tests), `pnpm build` — pass.
- i18n: `node scripts/check-i18n-completeness.js` + cross-face test pass;
  new web keys registered in `scripts/i18n-cross-face-contract.json`.
- `go run ./sdk/internal/cmd/source-hash internal ""` digest refreshed
  (3c8896ec…) in `sdk/internal/assembly/conformance_results.json`;
  `go test ./sdk/internal/ ./sdk/internal/conformance/` pass.
- Recorded real-journal ordering: `model.call.finished` may precede
  `model.completed`; message rows attach to the latest call attempt.
