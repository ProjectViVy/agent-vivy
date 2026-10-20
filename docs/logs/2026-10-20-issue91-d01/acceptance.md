# Acceptance

**Current status: implemented, not accepted.** The focused real-App regression
and independent code review pass. Full just ci fails; conformance evidence
must be regenerated successfully for the changed source, and remaining
full-gate failures must be resolved before accepting D01. See verification.md.

Run `TestAppRunsWithoutDigestAudit` through the existing Go test entry point.
The actual App/Service/RPC path must read an isolated file whose random nonce
was not in the user prompt, send the exact result with the same tool-call ID
to the next provider request, persist the final answer, journal one completed
terminal and return to idle. No digest-only debug audit stream is emitted.

Source inspection must show that only the retired audit facility was removed:
generic hooks, Channel hooks, required Journal records and actionhost auditing
remain. Existing nudge failure/cancel/recovery tests stay green and no warming
or steering code is removed.

This accepts a bounded D01 cut only after recorded checks and review. Full
#91 hash removal, persisted-format conversions, EXP draft re-planning and
later first-party plugin extraction remain separate work. Rollback is the
focused Git revert; this cut has no data-format transition or release artifact.
