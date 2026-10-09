# Verification

Environment: Linux, Go 1.26.8, pnpm 11.19.0, just 1.43.1; pinned Laputa
`ff3936f44ff8cf08c12af2cf698c194cfe474fd3`.

## Regression proof

- On the original dispatcher, the exact-order burst failed with
  `notification 246 = 255, want wire order`; the blocked-handler test failed
  with `model.delta overtook blocked model.request`.
- `go test -overlay /home/ubuntu/Downloads/vivy-rpc-before-overlay.json
  ./sdk/facerun -run TestRunVerifiesModelIntegrityThroughOrderedRPCNotifications
  -count=1` replaced only the transport implementation with `HEAD`'s original
  source. It failed: output length 6657, expected 6658. The working tree was
  never reverted. The same regression passed with the fix.

## Passing checks

| Command | Result |
| --- | --- |
| `go test ./internal/rpc -run 'TestPeer\|TestNotificationBuffer' -count=20` | Pass |
| `go test ./internal/rpc ./sdk/facerun -run 'TestPeer\|TestNotificationBuffer\|TestRunVerifies' -count=20` | Pass |
| `go test -race ./internal/rpc ./sdk/facerun -run 'TestPeer\|TestNotificationBuffer\|TestRunVerifies' -count=20` | Pass, no race reports |
| `go test -race ./internal/rpc ./sdk/facerun -count=1` | Pass, including existing control-plane and WebSocket tests |
| `go vet ./internal/rpc ./sdk/facerun` | Pass |
| `go test ./internal/app -run 'TestHeadlessTurnCompletesWithScriptedModel\|TestHeadlessSinkRendersStream' -count=1` | Pass; real scripted headless path |
| `go generate ./internal/generated/assembly` | Pass; no generated diff |
| `go test ./sdk/internal/conformance -run TestCheckedInProviderConformanceMatchesExecutedSuites -count=1` | Pass after pin refresh; all release suites reproduced |
| `go test ./sdk/internal -run 'TestGenerationFailureMatrixExecutesEveryCase/module-conflict' -count=1` | Pass |
| `gofmt` on all changed Go files; `git diff --check` | Pass |

## Full repository gate

`GIT_CONFIG_GLOBAL=/dev/null just ci` passed with exit code 0 on the final
source and pins: pinned-source bootstrap, formatting, UI typecheck/tests/build,
cross-face I18N, whole-repository vet/tests, headless compilation, and the
independent plugin/face module gates. No failures remained.

The first full run failed on stale conformance source pins, plus a matrix
source-hash mismatch while the source was still being changed. No gate was
disabled; pins were derived from the final tree and the producer gate rerun.
An overbroad duplicate full-package race run with `-count=20` was stopped;
the complete packages passed once and the affected regressions passed 20 times.

## Limits

No browser UI or live-provider test was needed: no UI/provider code changed.
The deterministic JSONL face and scripted headless path are the real-path
smokes. No Postgres DSN-backed tests or release build were requested. Hooks
were inspected; this checkout has no active pre-commit hook.
