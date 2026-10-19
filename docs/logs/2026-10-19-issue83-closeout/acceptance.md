# Human acceptance

Review the final draft PR against the candidate SHA in `verification.md`. Confirm that the latest-main development-environment changes remain present and inspect the recovery tests for cancellation-write failure, restart after Run terminal persistence but before Plan cancellation, repeated Recover, and exactly-once terminal/cancellation events.

For upgrade acceptance, use the actual historical and candidate binaries with the isolated SQLite database matrix. The Eino v0.9.13 historical binary must create pending Approval, Question, and Plan requests; the v0.9.21 candidate must fail each incompatible Run once, cancel each request once, preserve checkpoint id/generation/checksum, avoid model calls, and remain idempotent after restart.

For rollback acceptance, distinguish the unpatched historical binary from the compatibility target. The unpatched `e960929` binary must fail closed on migration 44 both with pending requests and after candidate cleanup, without changing request, event, or checkpoint state. This is a negative control and does not establish rollback support. Build the compatibility target with `scripts/build-eino0913-rollback.mjs`; verify its embedded Eino version is v0.9.13. The compatibility binary must retain the current recovery repair and migration 44, cancel pending v0.9.21 Approval/Question/Plan requests exactly once, preserve checkpoint pointers, avoid model calls, and pass repeated-restart checks. No checkpoint conversion or version-check bypass is accepted.

Confirm the PR remains unmerged and undeployed. Review GitHub checks for the exact candidate SHA separately from local Windows/Linux validation. The full AgenticModel migration and Provider milestones B5/B6/B7 remain open; this delivery must not claim their closure.
