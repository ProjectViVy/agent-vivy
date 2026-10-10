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

Real interaction follow-up: tests reproduced RED for question cancellation retries after queue replay/terminal failure and for approval cancellation stealing an already approved durable decision. The new real question/approval tests include one cancellation lifecycle event and exactly one terminal after retry; decision-winner controls remain active. Optional dequeue-ID test reproduced stale inspection removing a newer item. Final focused command covering those cases and existing pending/question/approval cancellation behavior passed (2.678s).

Sealed-carrier follow-up RED evidence: after failed admission, QueueRemove returned storage.ErrRunClosed; oversized full attachment DTOs were acknowledged; SQLite and live PostgreSQL session deletion left synthetic removal records. All corresponding focused regressions passed after the fixes. Control replay tests cover iterator error and close failure, private retry, and ignoring a previous carrier's controls after re-admission. Failed synthetic control writes retain the entire queue; successful retry is durable. A bounded 20 KB image dequeues intact and all its encoded journal events remain within the configured ceiling.

Focused runtime + SQLite + live PostgreSQL command passed (0.420s / 0.061s / 0.403s); final control-write failure and bounded-image regression passed (0.126s). The temporary PostgreSQL 17 test database supplied by root uses unique disposable test schemas; no production database was used. A full runtime/RPC/SQLite/live PostgreSQL rerun is also running for this checkpoint; final integrated CI remains root-owned.

Final sealed-carrier full rerun: go test -p 2 ./internal/runtime ./internal/rpc ./internal/storage/sqlite ./internal/storage/postgres -count=1 with VIVY_POSTGRES_TEST_DSN set: PASS (runtime 57.681s; RPC 15.318s; SQLite 7.147s; live Postgres 37.592s). git diff --check: PASS.
