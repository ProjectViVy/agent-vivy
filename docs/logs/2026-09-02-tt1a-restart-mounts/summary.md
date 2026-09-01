# TT-1a: restore skill mounts on restart recovery (journal replay)

## What changed

Closes the restart half of the two-tier-tools mount story (TT-2's leftover:
"跨重启恢复仍需 TT-1"): the live mount registry is memory-only, and restart
recovery used to rebuild `pendingRun` with `nil` mounts — a run suspended
after `skill_view` mounted hidden tools lost them all across a restart and
the resumed run could no longer call them.

- `internal/runtime/service.go`:
  - New `recoveredMounts(ctx, runID)`: replays the run's journal and
    accumulates the `tools` lists of `tool.mounted` events (TT-3) in order.
    Mounts are add-only within a run, so the replay reproduces the
    suspend-time set exactly. Returns nil when the run mounted nothing —
    the same shape recovery produced before — and logs (never fails) on a
    replay/decode problem: an unreadable payload degrades to "no mounts",
    not to a failed recovery.
  - `rebuildPending` and `rebuildPendingQuestion` seed `pendingRun.mounted`
    from the replay; `resumeRun` already rebinds the registry from the
    pendingRun (TT-2), so no resume-path change was needed.
  - The stale "restart recovery rebuilds pendingRun without one" comment on
    the field is replaced by the new contract.

Not done here: TT-1's session-level pin (mounts surviving *across runs* of
a session) is untouched — it needs the session-dimension storage and audit
scope decisions the TODO row names. This slice only makes an interrupted
run's pre-restart mounts survive its own resume.

## Files

- `internal/runtime/service.go` — recoveredMounts + two rebuild sites
- `internal/runtime/toolmount_resume_test.go` — restart acceptance test
