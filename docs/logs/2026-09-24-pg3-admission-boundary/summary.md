# PG-3 Task 1: Admission boundary

This iteration routes every primary producer through the existing per-session startup gate and linearizes durable admission against registered human intent. Authenticated RPC turns/edits, channel turns, action `StartRun`, and headless turns register human intent; Goal and Cron remain automatic. An automatic primary that meets a pending human intent yields the existing conflict, while already-committed primary conflicts remain unchanged.

Repeated Goal wakes are coalesced into one process-local worker and a single pending bit per session. A wake received while a candidate is checking eligibility is re-evaluated after that attempt, so a concurrent durable resume is not lost. The existing atomic `CommitGoalRun`, work-version CAS, Service activation, and Eino runner remain the sole execution path. No queue, ticket, second runner, Journal, schema, or dependency was added.

Changed implementation/test files: `internal/app/app.go`, `internal/app/headless.go`, `internal/runtime/service.go`, `internal/runtime/goal_driver.go`, `internal/runtime/human_admission_test.go`, and new `internal/runtime/primary_admission_test.go`. The checked-in SDK conformance evidence digest was refreshed for the changed `internal/` tree. This change does not include the two pre-existing modified UI generated files.

Explicitly out of scope: PG-0/PG-1 live PostgreSQL acceptance, product/model/browser evidence, PG-3 lifecycle/recovery tasks, and any Eino version upgrade.
