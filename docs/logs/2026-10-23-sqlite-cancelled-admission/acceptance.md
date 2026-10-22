# Acceptance

Run `TestJournalAppendPreservesCancelledTransactionCause` in
`internal/storage/sqlite`. A cancelled transaction that returns
`sql.ErrTxDone` must retain both that identity and `context.Canceled`.
An active-context transaction and a genuine commit failure must keep their
original failure classification. Failed appends persist no events; the next
append starts at sequence1 and reopening continues at sequence2.

Run the focused warming cases from verification.md and the required kernel
gates. Normal streaming warming must complete, mandatory persistence failures
must still fail the Run, and cancelled paid calls must drain before idle.
No live provider or production database is needed for these checks.

This correction does not close the sixteen baseline repair intake items or
the separate human Tiny Face checklist. It does not restart or replace the
owner's running manual-test package.
