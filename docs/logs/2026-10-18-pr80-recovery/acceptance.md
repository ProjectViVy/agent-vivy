# Acceptance

1. With a real historical Eino v0.9.13 binary, pause one Run on each of
   Approval, Question and Plan review, wait for durable checkpoints, then kill
   the process. Start the fixed v0.9.21 binary on that isolated database:
   every Run fails once with the engine-change message, every pending request
   is cancelled, cancellation evidence is complete, and restart changes none
   of those event counts. Recovery makes no provider calls.
2. Inject failures before row cancellation, between row and cancellation event,
   at terminal append, after terminal before Plan cancellation, and at intent
   creation/deletion. Recover reports the original cause; a fresh Service
   completes the cleanup. Repeated Recover leaves exactly one cancellation
   event and one terminal. A terminal Run does not hide unfinished Plan cleanup.
3. A direct historical rollback with pending v0.9.21 requests is an expected
   failing control, not supported use. For supported rollback, stop new work,
   finish or cancel under the current binary, verify all active/pending lists
   and cleanup intents empty with terminal/cancellation evidence, restart the
   current binary to recheck, then start the unchanged historical binary.
   Nothing may reappear as pending or gain another terminal/cancellation.
4. Checkpoint envelopes/payloads remain unchanged and mismatched versions are
   refused. This PR neither resumes cross-version checkpoints nor migrates
   the AgenticModel representation.
5. Review current candidate evidence in verification.md. A locally passed
   command is not a remote CI pass; unavailable browser checks and remaining
   B5/B6/B7 are recorded explicitly. No merge/deployment is part of acceptance.
