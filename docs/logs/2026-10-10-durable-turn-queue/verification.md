# Verification

Go commands use /workspace/agent-vivy/work/toolchain/go/bin, GOMAXPROCS=2, and go test -p 2. Native steering uses the existing pinned Eino ResumeWithParams, ChatModelAgentResumeData.HistoryModifier, actual agent interrupt context IDs, and adk.WithCancel/CancelAfterChatModel; source inspection confirmed the history modifier runs asynchronously, permitting the durable-consumption gate before model driving.

Regression RED evidence was recorded for ordered reactivation, queue storage errors, replay retry, pre-drive admission markers, attachment-bearing busy routes, immutable option snapshots, atomic steer transcript persistence, missing JSONL options, repeated phase cancellation, and multi-item clear rollback. The tests failed on the intended behavior before the corresponding fixes. Scratch logs remain under ignored work/.

Verified checkpoint:

- Full impacted Go packages (runtime, RPC, SQLite, Postgres, TUI live, JSONL face) passed once before the final cancellation/clear regressions. A subsequent RPC run panicked on a nil test-model reader; a fresh full RPC rerun passed (19.477s). This intermittent test fixture failure is reported rather than hidden.
- Targeted queue replay, admission failure, cancellation marker ordering, missing-checkpoint fallback, and repeated native steer/context regression passed (0.633s).
- Parked cancellation replay/commit retry and removed-snapshot admission regression passed (0.124s).
- UI TypeScript check passed. Full UI Vitest passed 77 files / 592 tests from ui/; an earlier run from repository root failed 20 i18n tests because their relative script path resolved from the wrong directory.
- After final UI cleanup, TypeScript passed and changed queue UI tests passed 3 files / 84 tests.

The final core checkpoint and impacted Go reruns are recorded below after completion. PostgreSQL tests compile and run their available local checks; live PostgreSQL persistence parity requires an integration database, which this lane does not have configured. Root owns just ci, race/integration checks, source hash refresh, packaging, and real browser smoke after combining the dependent lanes.

Final checkpoint results:

- go test -p 2 ./internal/runtime -run 'TestClearQueueFailure|TestPendingCancellationRetries|TestQueueAdmissionRejects|TestSteerResume|TestSteerMissing|TestConcurrentQueue' -count=1: PASS (0.392s).
- go test -p 2 ./internal/runtime ./internal/rpc ./internal/storage/sqlite ./internal/storage/postgres ./sdk/tui/live ./sdk/tui/face -count=1: PASS (runtime 43.320s; RPC 14.796s; SQLite 7.427s; Postgres 0.006s; live 4.301s; face 0.023s).
- git diff --check: PASS.
