# TFLAKE-CRON — Deterministic tests for the cron delete-after-run contract

## What changed

`TestCronAtJobDeletesAfterSuccessfulRun` occasionally timed out under full-load
`just ci`: the test waited end-to-end for the asynchronous
fire→run→watch→settle pipeline to delete within the wall-clock budget, so going
over budget under load became a false failure (mitigated on 2026-08-31: budget
5s→30s plus a dump of settled rows on failure).

The durable fix moves the check to the contract layer: delete-after-run is
implemented by the synchronous branch of `settleCronRun` (`cron_scheduler.go`
AT + DeleteAfterRun + ok → DeleteCronJob), so it no longer depends on
wall-clock verification:

- Added `TestCronSettleDeletesSuccessfulAtJob`: directly constructs a
  `cronActiveRun`, calls `settleCronRun(RunCompleted)`, and synchronously asserts
  that the row is deleted and the active marker is cleared. No scheduling loop,
  no wait.
- Added the failure twin `TestCronSettleKeepsFailedAtJobDisabled`: a failed
  `delete_after_run` one-shot job **must not be deleted** (the operator needs the
  error state on the row); it asserts retention + disabled +
  `last_status=error` (a representative failure path).
- The existing end-to-end test remains as a wiring canary (including the
  unchanged 30s budget mitigation).

No production-code changes; test strengthening only.

## Explicitly not done

- No fake clock was introduced (`CronSchedulerOptions.Now` already exists for
  callers that need it; this slice's flake source is the pipeline wall-clock
  budget, not timer precision, and after the contract unit test the canary's
  intermittent timeout no longer masks real regressions).
- The production logic in `settleCronRun` was not changed.
