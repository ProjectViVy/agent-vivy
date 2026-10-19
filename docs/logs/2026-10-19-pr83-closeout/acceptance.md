# Acceptance

Acceptance record for source SHA `8ec876d3035c0ed11e5ac5463b3e5abc29565506` only. The integrated issue #83 candidate's rollback acceptance is documented separately in `docs/logs/2026-10-19-issue83-closeout/acceptance.md`.

For an upgrade from the historical Eino v0.9.13 binary, the fixed v0.9.21
candidate recovers paused Approval, Question, and Plan work by failing each
incompatible Run once and recording the corresponding request cancellation
once. It preserves the existing checkpoint bytes and does not invoke the model
while recovering.

An unpatched historical binary cannot consume the new cleanup intent. Do not
start it while any incompatible request or Plan review remains pending. The
supported historical rollback procedure is:

1. Stop admitting new Runs and use the fixed candidate to finish, decide, or
   cancel all active/paused work.
2. Verify there are no active Runs, pending Approvals, pending Questions,
   pending Plan reviews, or cleanup intents. Check that every affected Run has
   exactly one terminal and the cancellation events are present exactly once.
3. Restart the fixed candidate and repeat those checks; keep it running and retry
   if any cleanup write or verification fails.
4. Only then stop the candidate and start the historical binary.

The actual Windows binary matrix verifies both this procedure and the unsafe
direct-rollback negative control. It does not convert checkpoints or bypass
version checks. No compatibility rollback build is provided. Full AgenticModel
migration and Provider milestones B5/B6/B7 remain outside this acceptance.
