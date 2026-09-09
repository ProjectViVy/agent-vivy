# TT-1 session-level pin (legacy table, no new table)

## What changed

- **Storage**: add `ListRunsBySession(ctx, sessionID)` to the `RunStore` contract
  (`internal/storage/contracts.go`) — query the **existing runs table**, with no new table
  (decision: legacy table). Implement both sqlite/postgres backends (`listRunsWhere` shared
  predicate helper, `ORDER BY created_at, id` matching existing convention, all statuses in
  creation order); conformance suite CN-19 `runs listed by session` (creation-order
  assertion + all-status assertion + unknown-session empty-set negative case), suite guard
  18→19.
- **Runtime**: `Service.sessionMounts` (`internal/runtime/service.go`) takes over at the
  drive assembly point: after creating the run row, list prior runs by sessionID and replay
  each journal to collect `tool.mounted` payloads (reusing TT-1a's `recoveredMounts` parsing
  pattern), accumulate them in creation order as a `MountedTools` seed for the new run (skip
  the current run because no tools have been mounted yet). **No resume-path changes** (TT-2
  prioritizes the captured snapshot, and `resumeRun` still uses `pendingRun.mounted`).
- **Best-effort semantics**: a listing failure or journal-replay failure for an individual
  run → warn + continue with the portion collected so far (or empty); never fail the run.
  A corrupt payload degrades to a warning and skip, as with `recoveredMounts`.
- **Governance unchanged**: the mount-admission gate (tooladapter) and approval path are
  untouched — pin restores only "callability"; calls to hidden tools remain governed by
  policy/approval as usual.
- **Tests** (`internal/runtime/toolmount_session_test.go`):
  - `TestServiceSessionPinRestoresSkillMountedTools`: run1 mounts echo_info through
    `skill_view` → completes; run2 in the same session calls echo_info directly and succeeds,
    with no `skill_view` call in run2 (there is no script for it).
  - `TestServiceSessionPinDoesNotLeakAcrossSessions`: with the same storage, a run in another
    session calling echo_info is rejected by the admission gate — the engine node stops with
    an error and run is `failed` (negative-case semantics stronger than "reject replay").
  - Discriminating check: after removing seeding in the drive (probe
    `(*tools.MountedTools)(nil)`), run2 cannot complete; restoring it makes both tests green.
- **Incidental fix**: `TestVC1Walkthrough` timed out under `-race` — the shared
  `waitForRunStatus` 5s deadline was too tight (bash steps spawn real processes one by one,
  taking >5s under race; the DB was closed during test-failure cleanup and the run completed
  later with an ERROR log). Use a test-specific `waitForWalkthroughStatus` (60s deadline,
  20ms polling).

## Explicitly not done

- Do not add session-dimensional mount "unmount" or audit UI: pin is an add-only convenience
  semantic within a session, and the `tool.mounted` event stream (TT-3) is already the audit
  source of truth.
- The child-run drive path does not use pin: the child tool surface is narrowed to a
  read-only subset; whether to inject the session pin into a child belongs to the later VC-2
  agent-tool semantics decision and is not expanded here.

## Notes

- Performance: seed cost = number of runs in the same session × journal replay. The drive runs
  asynchronously inside the run goroutine and does not add RPC latency; for long sessions,
  replay is a sequential scan filtered up to `tool.mounted` events, and session run counts
  are usually in the tens, which is acceptable. If it becomes a hotspot, add a type index to
  the journal (not done, to stay within the legacy-table decision scope).
- A child run's journal also carries the same sessionID, so its `tool.mounted` events are
  absorbed into the seed of later main runs — consistent with the "mounts only grow within a
  session" semantic.
