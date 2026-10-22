# Preserve cancelled journal-admission causes

Stable-push verification at `c02c4b30` exposed a failed
`TestCacheWarmStreamingFiresAfterSettle`: a later `model.request` append
returned only `sql.ErrTxDone`, and the Run failed instead of completing.
The test had already observed a successful warm before releasing the final
foreground response. Twenty focused repetitions subsequently passed unchanged;
the original timing window is not deterministically reproduced.

The source-supported mechanism is final model settlement scheduling another
warm while Run drainage cancels its context. Go's `database/sql` can roll back
a context-bound transaction before `Commit` and return only `sql.ErrTxDone`.
The existing maintenance-admission guard recognizes `context.Canceled`; losing
that cause converts an unadmitted shutdown into a mandatory-observation error.
This fixture supplies its own model, bypassing the custom-provider repair.

SQLite `Append` now joins the context cause only when `Commit` returns
`sql.ErrTxDone` and that transaction's context has ended. The original error
remains inspectable. Active-context `ErrTxDone` and unrelated persistence
errors remain failures. No request is retried, no paid model call bypasses
admission, and no transaction, migration, schema or runtime policy is changed.

A deterministic regression executes actual Append SQL and real SQLite
rollback, injecting the error shape at the driver's commit boundary. It
checks cancelled/active transactions and genuine storage failure during
cancellation, zero persisted rows, and contiguous sequences after successful
append/reopen. It proves error propagation, not the original watcher timing.

An independent read-only source review accepts the two-file correction with
no blocker. Native RED/GREEN and focused runtime checks pass. Final Linux
integration gates are recorded separately as they complete; no stable push
or D14 closure is claimed by this repair record alone. Release notes are
omitted because this is a source correction, not an artifact release.
