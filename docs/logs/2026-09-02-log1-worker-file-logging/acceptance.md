# Acceptance — LOG-1

## How a human can tell it worked

1. Build and start Vivy as usual (`just run` or the installed binary).
   No config change is needed: worker logging inherits the existing
   `logging` block (`level`, `format`, `dir` default
   `<data_dir>/logs`).
2. Trigger anything that spawns a worker child (a child run via the
   RPC control plane).
3. Look into the log directory: next to the usual
   `vivy.log.YYYY-MM-DD` files there is now one
   `vivy.log.worker-<pid>` file per spawned worker, containing at
   least `worker started` (with `pid` and the file `path`) and, when
   the child ends, `worker ended`. Level/format follow the parent's
   effective settings, including `VIVY_LOG_LEVEL` overrides.
4. After the parent restarts, worker files older than
   `retention_days` are pruned together with the parent's rotated
   files (same `vivy.log` prefix sweep).

## Regression guarantees

- Without the handoff (e.g. `vivy worker` started by hand with no
  `VIVY_WORKER_LOG_*`), the child behaves exactly as before: no file
  sink, protocol errors only via RPC/stderr, nothing on stdout.
- Worker JSONL protocol traffic never lands in the log file; the only
  stdout consumer remains the protocol.
- The daily parent sink, rotation, and sweep behavior is unchanged
  (`Setup` refactor is behavior-preserving; `TestSetupDefaultsAndFileSink`,
  `TestSetupTextFormat`, `TestSetupEnvOverrides`,
  `TestDailyFileRollover`, `TestCleanOldLogs` all still pass).
