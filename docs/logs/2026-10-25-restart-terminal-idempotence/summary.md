# Restart terminal idempotence: reconcile the run row when the terminal is already durable

Date: 2026-10-25 · Branch: `fix/restart-terminal-idempotent` (from `fix/tool-failure-continuation` @ `e858f878`)

## What changed

Observed 2026-10-24 while diagnosing child failures (`child_2c0394b98e6c04c9`),
boarded as `RUN-RESTART-DUPLICATE-TERMINAL`: startup recovery logged

```text
journal append of terminal event failed ... err="storage: begin append: database is locked (5) (SQLITE_BUSY)"
```

and then, on every later start,

```text
journal append of terminal event failed ... err="storage: run already has a terminal event"
```

for the same run. The first append had landed the terminal event but the process
never reached `SetRunStatus`, so the run row stayed `active` forever. Recovery
enumerates non-terminal rows, so each restart re-drove the same run and hit the
closed journal again — a permanent, self-repeating error with no data loss but no
way out.

Two changes, one delivery:

1. **`emitTerminal` reconciles instead of giving up** (`internal/runtime/service.go`).
   The append error branch now distinguishes `storage.ErrRunClosed` (the sentinel
   `internal/storage/sqlite/journal.go` returns once a run holds a terminal) from a
   genuine backend failure. On `ErrRunClosed` it reads the recorded terminal back
   from the Journal through the new `recordedTerminal` helper, adopts its type and
   seq, flips the run row with the same `SetRunStatus` call the healthy path uses,
   and then walks the identical cleanup. The Journal is the storage truth: when the
   recorded event and the in-memory one disagree, the recorded one wins.
   The reconciliation **skips `s.publish`** — the event was published when it was
   first appended, and republishing a terminal would break the exactly-once replay
   contract (AS-7); RPC subscribers already fall back to the journal replay. The log
   drops from `Error` to `Info("terminal already recorded; reconciled run row")`.
   Any other append error keeps today's behavior: log, clean the in-memory state,
   return without touching the row.
2. **SQLite waits for the write lock** (`internal/storage/sqlite/sqlite.go`). The DSN
   gained `_busy_timeout=5000`. `SetMaxOpenConns(1)` (D-027 single writer) only
   serializes this process; a second handle — a lease-holding neighbour, a backup
   reader — can still own the write lock, and the default zero timeout turns that
   into an immediate `SQLITE_BUSY`. Five seconds makes the occasional contention a
   wait instead of a failed terminal append. D-027 is unchanged: still one
   connection, no WAL or locking knob leaks through the storage contracts.

`_busy_timeout` is the pinned driver's own DSN key (`modernc.org/sqlite v1.59.0`,
`dsnPick(q, "_busy_timeout", "_timeout")`, validated as an integer), not a
mattn-ism; the value is a named constant `busyTimeoutMS` beside the lease knobs.

## Explicitly not done

- No change to the recovery enumeration itself. A run whose row is terminal is
  simply no longer listed; the reconciliation happens on the path that already owns
  the terminal.
- No second terminal owner and no new "already closed" state. One writer, one
  terminal per run (D-008) is untouched.
- No change to `publish` semantics, the bus, or the AS-7 replay contract.
- No log-capture assertion. The runtime test suite has no `slog` capture helper, and
  installing a process-global handler to assert one line would race every parallel
  test; the observable contract (row reconciled, exactly one terminal, second
  recovery silent) is asserted instead.
- `PostgreSQL` needs no equivalent: its DSN carries no busy-timeout knob and its
  lock waits are already bounded by the connection settings.
