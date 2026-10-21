# Human acceptance

1. Select authorized history records, preview them and attach. The committed
   excerpt must contain exactly the previewed positions/text, with no digest row
   and no adjacent source content.
2. Change selected text/positions, delete or rewind the source, or revoke its
   scope before attachment. Attachment must fail without a destination copy;
   the draft remains recoverable. The same source changes at commit are fenced.
3. Queue a submission, then change the composer draft. The queued expected
   records stay unchanged; sending stale records conflicts rather than silently
   refreshing them. Repeating a committed request replays its saved copy.
4. Reopen/fork/compact/delete a source. Existing destination-owned copies and
   imported-user-data projection keep their identities and established lifecycle.
   Historical Journal bytes are preserved; retired digest input is rejected.
5. On PostgreSQL, competing subtree deletion and attachment must not deadlock.
   Membership changes while acquiring locks cause a retryable conflict before
   deletion writes, with every session retained.

These are reproducible instructions, not a claim of human execution. Owner-
deferred3015 interaction, durable presentations/downloads and final acceptance
remain open in the sole Story index.
