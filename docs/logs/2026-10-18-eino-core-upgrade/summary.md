# Eino core upgrade v0.9.13 → v0.9.21 with a pending-run policy

Phases P0 and P1 of the provider-agentic program
(`docs/plans/provider-agentic/S3-migration-inventory.md` §7.6), after the
owner approved the upgrade on 2026-10-18 (S4 blocker B1).

## What changed

- **Dependency:** `go.mod` pins `github.com/cloudwego/eino v0.9.21` (was
  v0.9.13); `go.sum` gains its two hashes. Nothing else moves: the current
  `components/model/{openai,claude}` generation and every other pin stay.
- **Restart recovery bug fixed** (`internal/runtime/service.go`). When a paused
  run's checkpoint could not be read — always the case after an Eino version
  change, which is why `EINO-CORE-UPGRADE` was deferred — recovery marked the
  run failed but left its approval or question `pending` and its Plan review
  pending. It now:
  - cancels the pending approval/question before the terminal event
    (existing `cancelApproval`/`cancelQuestion`), and the Plan review after it
    (existing `cancelPlanReviewForRun`);
  - on `ErrCheckpointEngineVersionMismatch`, closes the run with a specific
    message ("Vivy was updated while this run was waiting…") instead of the
    generic restart text.
  `checkpointReadable` now wraps a new `checkpointReadError` that keeps the
  cause; `failUnrecoverable` delegates to `failUnrecoverableWithMessage`.
- **Policy (P0)** written into the HITL contract:
  `docs/architecture/hitl-review-center.md` "Restart across an engine version":
  no drain required, no checkpoint conversion, fail-closed version check kept,
  pending interactions cancelled, user-facing message defined.
- **Tests:** `internal/runtime/recovery_engine_version_test.go` (approval,
  question, Plan review across a version change; approval with a missing
  checkpoint at the same version). Restart helpers gained an engine-version
  parameter (`restartServiceAt`, `restartQuestionServiceAt`).
- **Backlog:** `EINO-CORE-UPGRADE` and `EINO-TOOLSNODE-ERR-RACE` moved from
  `docs/DEFER.MD` (and the duplicate open row in `docs/TODO.md`) to
  `docs/COMPLETE.MD`. Spike docs (S3 phases, S4 B1, README) updated.

## Not done

- Child runs are unchanged: their recovery already denies pending approvals and
  uses the child "worker lost" message.
- No live-provider run with a real paused approval across two binaries (no
  credentials); the path is covered by the unit tests above and the
  two-binary boot smoke in `verification.md`.
- Provider migration phases P2–P4 are not started (blockers B3–B7).
