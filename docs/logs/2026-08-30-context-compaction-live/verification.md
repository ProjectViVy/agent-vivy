# Verification record (2026-08-30, live context compaction)

## Commands and results

| Stage | Command | Result |
|---|---|---|
| Compile | `go build ./...` | ✅ 0 errors |
| Go tests (first full run) | `go test ./...` | ✅ All green (including `internal/runtime` 42.5s and `internal/rpc` 21s) |
| New runtime tests | `go test ./internal/runtime/ -run 'TestCompactionPolicyTriggerTokens\|TestCountMessageTokens\|TestMapperMapsSummarizationUsageEvent\|TestEngineSummarizationCompaction\|TestEngineReductionRunsBeforeSummarization\|TestServiceContextStatusAndCompactSession\|TestScheduleEngineReload' -count=1` | ✅ 7/7 passed |
| Settings-overlay tests | `go test ./internal/app/settings/ -run Compaction` | ✅ Passed |
| RPC tests | `go test ./internal/rpc/ -run TestContextCompactionRPC` | ✅ Passed |
| Config tests | `go test ./internal/config/` | ✅ Passed |
| Frontend types | `pnpm run typecheck` (ui/) | ✅ No errors |
| Frontend tests | `pnpm test` (ui/) | ✅ 21 files / 177 tests |
| Gate | `just ci` | ✅ All green: gofmt / `go vet ./...` / `go test ./...` / headless build / UI install + typecheck + test (21 files, 177 tests) + vite build |
| Real-device startup smoke | Start `go build ./cmd/vivy` with a temporary mock config (`runtime.mock: true` + `compaction.enabled: true`) and call `GET /rpc/bootstrap` | ✅ HTTP 200 (complete root composition: migration 015 plus an engine with summarization/reduction assembled, started, and listening) |

The first `just ci` run failed at `fmt-check` (new files were not gofmt-formatted); it
was rerun after `gofmt -w` fixed them.

### Notes

- Engine-wiring tests use `ScriptedModel` + `recordingModel`: assert the two model-call
  inputs when summarization triggers (summary-generation input > main-loop input, and
  the main-loop input contains the summary); assert reduction precedes summarization
  (the summary-generation input contains the `Old tool result content cleared`
  placeholder).
- Session-level tests use `fixedReplyModel` (the reply does not echo the input, because
  a mock echo would make the summary larger), asserting that `session_compactions` is
  persisted, the `context.compacted` event enters the Journal,
  `ContextStatus.has_compaction_summary=true`, and the feed folds from 40 rows to 6+1.
- `ScheduleEngineReload` tests assert: swap immediately while idle, defer while active,
  and apply after returning to idle.
- When reducer/summarizer share a trigger threshold, behavior is as expected: reduction
  clears only tool-result placeholders (retaining parameters), tokens may still be at
  the threshold, then summarization provides the fallback. The ordering test pins this
  pipeline.

## Unrun items (recorded per process)

- This environment did not start the split pair or browser, so the browser-click smoke
  at `http://127.0.0.1:3015` was not run; unit/integration tests plus real-device
  bootstrap smoke cover the main paths. See `acceptance.md` for steps that can be
  walked through 1–5 after `just dev`.
