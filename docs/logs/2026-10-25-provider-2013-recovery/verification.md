# Verification

All commands ran from the repository root on macOS with the pinned toolchain
(`GOTOOLCHAIN=go1.26.4`). No test reaches a live provider.

## Gates

| Command | Result |
|---|---|
| `gofmt -l internal/provider internal/runtime` | empty (no files listed) |
| `go vet ./internal/provider/... ./internal/runtime/...` | pass |
| `go test ./internal/provider/... -count=1` | `ok agent-vivy/internal/provider 0.462s` |
| `go test ./internal/runtime/... -run 'TestBare2013\|TestUnambiguousFirstRequest\|TestToolResultOrder\|TestPairToolTurns\|TestBuildRunContext' -count=1` | `ok agent-vivy/internal/runtime 0.789s` |
| `go test ./internal/runtime/... ./internal/app/... ./internal/provider/... -count=1` | first run: `internal/runtime 69.920s`, `internal/runtime/compaction 0.079s`, `internal/app/settings 1.020s`, `internal/provider 0.420s` pass; `internal/app` **failed** on `TestFailedModelCallRecordsContentFreeWireSummary` (below) |
| `go test ./internal/app/... -count=1` (after the fix) | `ok agent-vivy/internal/app 19.345s`, `ok agent-vivy/internal/app/settings 0.835s` |
| `go build ./cmd/...` | pass |
| `git diff --check` | pass |

## Regression found and fixed inside this delivery

`internal/app/assembly_audit_test.go:TestFailedModelCallRecordsContentFreeWireSummary`
simulated the real failure (a `400` body `invalid params, 400 (2013)` on the
first turn) and asserted **one** captured request. That input is exactly the
case this delivery now replays, so the fake server legitimately received two
byte-identical requests and the assertion failed with
`want one request carrying the nonce, got 2 requests`.

The test was corrected, not weakened: it now requires exactly two requests,
both carrying the nonce and byte-identical, and keeps every original
assertion (run fails, one v5 `model.request`, a content-free `wire` summary on
the failed finish). The fact that the replay is byte-identical is the property
the journal's fingerprint relies on, so the test now proves it end to end.

## Follow-up: child engines carried no retry decider

Found after the first commit while answering how a sub-agent executes: a child
run uses its own `Engine` from `ChildView`/`OneShotView`, which builds a new
`ChatModelAgent` and a new `ModelRetryConfig` closure, while the service bound
the decider only on the primary engine. No child model call was retried, so the
delivery's headline case (a child's first call) was still unprotected. The
outbound normalizer did apply, because a child view reuses the same wrapped
`model.AgenticModel`.

Fix and re-verification:

| Command | Result |
|---|---|
| `go test ./internal/runtime/ -run 'TestChildEngineInheritsModelRetryDecider\|TestChildModelRetry' -count=1` | 3 tests pass |
| `gofmt -l internal/provider internal/runtime internal/app` | empty |
| `go vet ./internal/provider/... ./internal/runtime/...` | pass |
| `go test ./internal/runtime/... ./internal/app/... ./internal/provider/... -count=1` | `runtime 70.574s`, `runtime/compaction 0.269s`, `app 19.750s`, `app/settings 1.064s`, `provider 0.708s` — all pass |
| `go build ./cmd/...` | pass |

## Not run

- `just ci` / `just wiring-ci` / `just kernel-ci`: not applicable to this
  provider/runtime change and not available on this host for the kernel gate.
- Live MiniMax child-subagent test: owner-run, needs a provider key. The
  expected observation is recorded in `acceptance.md`.
- Race detector: unchanged from baseline; this change adds no concurrency.
  `modelRetries` is guarded by the existing `Service.mu`.
