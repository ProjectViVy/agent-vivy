# Eino v0.9.21 upgrade and provider alignment backport

Date: 2026-10-19

Phase-A infrastructure preamble (E1 + E2) on branch `feat/issue-91-base`, cut
from the #91 planning branch. Backported from main commits `8f263d36` (Eino
upgrade with the pending-run policy) and `8ec876d3` (Claude stream-race patch).
This is dependency and recovery alignment, not a capability increment; the
owner approved it so the stable branch does not diverge on infrastructure
(decision 11 in
[owner decisions](../2026-10-19-baseline-a-epic-split/owner-decisions.md)).

## What changed

- **Dependency:** `go.mod` pins `github.com/cloudwego/eino v0.9.21` (was
  `v0.9.13`); `go.sum` gains its hashes. No other pin moves.
- **Pending-run policy** (`internal/runtime/service.go`): recovery now keeps the
  checkpoint read error (`checkpointReadError`, `errCheckpointMissing`) instead
  of a boolean, and `failLostCheckpoint` settles a suspended run whose
  checkpoint cannot be served back. An approval or question is cancelled before
  the terminal event; a pending Plan review is cancelled after it. On
  `ErrCheckpointEngineVersionMismatch` the run fails with the
  "Vivy was updated…" message (`engineChangedMessage`); a lost checkpoint at
  the same version keeps the generic restart text. `failUnrecoverableWithMessage`
  carries the caller-chosen text; `failUnrecoverable` delegates with the old
  message, so child-run behavior is unchanged.
- **Policy record:** `docs/architecture/hitl-review-center.md` gains
  "Restart across an engine version (pending-run policy)".
- **Tests:** new `internal/runtime/recovery_engine_version_test.go` covers
  approval, question and Plan review across an engine-version change, plus a
  same-version missing checkpoint. `restartServiceAt` /
  `restartQuestionServiceAt` are the engine-version-aware restart helpers.
- **Provider patch (E2):** `third_party/eino-claude` vendors the production
  sources of `eino-ext/components/model/claude` v0.1.25 plus one local race fix
  (per-worker `ConcatMessages` error variable), with the upstream Apache-2.0
  license and `PATCHES.md`. `go.mod` replaces only that module. The baseline-A
  Dockerfile copies the tree wholesale, so no Dockerfile line is needed here
  (main's per-module `COPY` lines do not exist at baseline A).
- **Citations and board:** every plan/Eino reference now reads `v0.9.21`;
  `EINO-CORE-UPGRADE` and `EINO-TOOLSNODE-ERR-RACE` moved from
  `DEFER.MD`/`TODO.md` to `COMPLETE.MD`.

## What was explicitly not done

- No provider bug-fix batch (E3): `f5322c92`, `1bdfdca4`, `773dc741`,
  `8a37e059` stay main-only until triaged against baseline-A contracts.
- No agentic P2-P4 work; the one-message-generation migration is not started.
- No checkpoint conversion, no drain, no schema or migration change.
- No `-race` run, no full `just ci`, no Docker build, no real-provider run in
  this change.
