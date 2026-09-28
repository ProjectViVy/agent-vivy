# MR-5: reconcile fixtures and writers with the new invariants

Defects: D11 (session-primary uniqueness), D12 (`messages.position`
allocation).

## Steps

1. Every `INSERT INTO messages` site allocates `position` under
   `lockMessageSession`: `internal/storage/sqlite/{runs,run_admission,
   work_control}.go` and `internal/storage/postgres/{runs,run_admission,
   work_control}.go`.
2. Fixtures predating one-active-primary-per-session: complete the fixture run
   (`SetRunStatus(..., RunCompleted)`) before starting the next primary run in
   `internal/runtime/service_test.go` and `reference_context_test.go`.
3. `AppendMessage` requires a session row: seed `CreateSession` first in
   `internal/channelhost/deliver_recovery_test.go` and
   `internal/rpc/control_test.go`.
4. `internal/storage/conformance/history.go` `assertHistoryRunLimit`: use
   terminal `RunCompleted` rows — the test exercises the narrow-scope cap, not
   run admission.

## Evidence

`internal/storage/sqlite`, `internal/rpc`, `internal/app` green; postgres
mirrors sqlite semantics (DSN-backed run remains a CI gate).
