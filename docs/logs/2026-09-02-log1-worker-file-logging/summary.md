# LOG-1 — per-worker file logging for `vivy worker`

## Summary

`vivy worker` child processes now write diagnostics to their own log
file instead of being invisible. The design resolves the documented
multi-writer blocker (LOGGING.md §7, TODO LOG-1) with **one writer per
file**: each child appends to `<log_dir>/vivy.log.worker-<pid>`, created
by the new `logging.SetupWorker`. There is no rotation, no sweep, and no
stdout output in the child (the JSONL worker protocol owns stdout).

Handoff: the supervisor (`internal/worker`) exports the parent's
*effective* log settings through the `VIVY_WORKER_LOG_DIR` /
`VIVY_WORKER_LOG_LEVEL` / `VIVY_WORKER_LOG_FORMAT` environment when
spawning the child. `app.New` resolves the effective values with the new
`logging.ResolveEffective`, which shares the exact env-override
precedence with `Setup` (one implementation, `resolveLevel` /
`resolveFormat`). Child precedence: worker env → inherited `VIVY_LOG_*`
→ defaults. An unset dir env keeps the previous sink-free behavior.

Pruning reuses the existing startup sweep for free: worker files share
the `vivy.log` prefix, so the parent's `retention_days` mtime sweep
deletes a dead worker's file on the next parent start.

## Changed

- `internal/logging/logging.go` — `SetupWorker` (per-worker sink, strict
  env parsing, disabled without dir), `ResolveEffective` +
  `resolveLevel`/`resolveFormat` (shared override resolution, `Setup`
  refactored onto them), `VIVY_WORKER_LOG_*` constants.
- `internal/worker/supervisor.go` — `Authority.Log` (`WorkerLog`
  handoff struct), `workerLogEnv` mapping, `cmd.Env` propagation in
  `StartWithBrokers` when a dir is configured.
- `internal/app/app.go` — resolve effective settings at composition and
  pass `worker.WorkerLog` into `newWorkerManager`.
- `internal/app/worker.go` — `workerManager.workerLog` field, filled
  into every child `Authority`.
- `cmd/vivy/main.go` — worker branch installs `SetupWorker`'s sink and
  logs the `worker started` / `worker ended` lifecycle lines (worker
  lifecycle logging lives in main, not the worker package, because
  `server_test.go` exercises `Run` in-process).
- `docs/architecture/LOGGING.md` — §1 names `SetupWorker` as the
  per-process-kind init path; §2 documents the `VIVY_WORKER_LOG_*`
  family as supervisor-internal (not an operator override); §3 gets the
  per-worker destination bullet; §7 drops the delivered LOG-1 item.

## Explicitly not done

- No log rotation in workers (parent sweep handles retention).
- No structured correlation between parent run ids and worker log lines
  (worker logs are ops scratch; the Journal stays the durable record).
- No UI/Studio surface for worker log files.
- Redaction defense-in-depth (LOG-3) is untouched.
