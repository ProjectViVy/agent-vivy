# Acceptance — restart terminal idempotence

How a human tells this worked, from the operator's point of view.

## 1. The repeating error is gone

Before: a run whose terminal append hit `SQLITE_BUSY` stayed `active` in the run
table, and **every** subsequent `vivy` start logged the same pair of lines for that
run — first `database is locked`, then `storage: run already has a terminal event`
on every later start, forever.

After: the first start that finds the stuck run logs, once,

```text
INFO terminal already recorded; reconciled run row run=<run-id> type=run.failed status=failed
```

and the run row becomes terminal. The next start logs nothing about that run,
because recovery no longer enumerates it.

Check: start the server twice over a database that contains a stuck run and compare
the two logs. The message appears at most once, and only on the run that reconciles
it.

## 2. No duplicate terminal, no lost terminal

The Journal still holds exactly one terminal event for the run (D-008). The
reconciliation never appends; it only reads back what is already durable and flips
the row.

Check: `run/log` for that run shows one `run.failed` / `run.completed` /
`run.cancelled`, and its `seq` is unchanged from before the restart.

## 3. The run's status matches the recorded event, not the in-memory one

If the durable terminal is `run.cancelled` but the recovering process was about to
write `run.failed`, the row ends up `cancelled`. The Journal is the storage truth.

Check: the Run Inspector / session run list shows the status that matches the
terminal event in the transcript.

## 4. Subscribers are not double-notified

The reconciled terminal is **not** republished. A UI that was already connected does
not see a second terminal frame for the same run; a UI that connects later reads the
event from the journal replay exactly once (AS-7).

Check: with the split pair running (`just run` + `cd ui; pnpm dev`,
http://127.0.0.1:3015), open a session containing a reconciled run and reload — the
transcript renders the terminal once, and no duplicate "run failed" row appears.

## 5. Nothing else moved

Ordinary runs still complete, fail, and cancel exactly as before; the healthy path
still publishes its terminal. The regression suite for recovery
(`TestServiceRecover*`), the goal-recovery shutdown path, and the SQLite storage
conformance tests are unchanged and green. The SQLite DSN is unchanged too: the
`_busy_timeout` knob this delivery briefly added was removed again (see
`summary.md`), so lock contention behaves exactly as it did before.

A stuck run whose first append hit `SQLITE_BUSY` still reconciles on the next start —
that is item 1, and it never depended on the timeout.
