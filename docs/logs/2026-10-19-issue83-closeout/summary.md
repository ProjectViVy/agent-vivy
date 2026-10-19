# PR #80 recovery closeout on issue #83 candidate

This candidate integrates the pending-run recovery repair with the selected issue #83 closeout work on latest main, while preserving main's development-environment changes. The recovery path persists cleanup intent before terminalizing an incompatible or lost-checkpoint Run, retries failed Approval/Question/Plan cancellation writes after restart, and keeps terminal and cancellation events idempotent.

The candidate upgrades Eino to v0.9.21 without converting checkpoints or bypassing exact-version checks. Scope also includes the selected #79/#82 integration, the existing #69 Skill bridge hardening, the compatible Laputa source pin, and the browser helper required by the integrated candidate. It does not complete the AgenticModel migration; Provider milestones B5/B6/B7 remain open. The PR is not merged or deployed by this delivery.

Upgrade from the historical Eino v0.9.13 binary to the fixed v0.9.21 candidate is verified for pending Approval, Question, and Plan requests. The unpatched historical binary from `e960929` cannot open a database after migration 44, even after pending requests are cleaned; it fails closed with `unknown migration version 44`. The supported Eino rollback target is a compatibility binary built from this same source with the recovery fix and migration 44 retained and Eino pinned to v0.9.13. The reproducible build is `scripts/build-eino0913-rollback.mjs`. It still enforces checkpoint version identity and cancels incompatible pending work instead of resuming it.

This rollback boundary supersedes the historical procedure recorded for the earlier PR #84 candidate in `docs/logs/2026-10-19-pr83-closeout/`; that record remains specific to its own source SHA.

See `verification.md` for the final candidate SHA, exact local commands and outcomes, actual binary hashes, remote checks, and remaining blockers.
