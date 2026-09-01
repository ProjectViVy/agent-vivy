# TT-2: restore skill-mounted tools on run resume — 2026-09-02

## Summary

Skill-declared tool mounts (TT-1 two-tier tools) were lost across a run
resume. `resumeRun` rebuilt the run context with the originally selected
tool surface but never rebound the run-scoped `tools.MountedTools`
registry, so any tool a skill had mounted before an approval/question
interrupt was rejected by the tool adapter after the decision
(`runtime: tool %q is not selected for this request`), and a `skill_view`
in the resumed segment silently mounted nothing.

Fix (one concern, kernel only):

- `pendingRun` gained a `mounted *tools.MountedTools` field; both suspend
  handlers (approval interrupt and question interrupt) capture the run's
  mount registry from the driving context.
- `resumeRun` takes the captured registry and rebinds it onto the resume
  context. A nil registry (restart recovery path, where mounts are
  memory-only and therefore gone) falls back to a fresh registry so a
  `skill_view` in the resumed segment can still mount new tools.
- Both resume callers (approval decision and question answer) pass the
  captured registry through.

Explicitly not done (stays with TT-3 / TT-1):

- Cross-restart restore of mounts: mounts are still memory-only. After a
  process restart the recovered run gets a fresh (empty) registry; the
  model must re-view the skill to re-mount. Journaling mounts is TT-3.
- Session-level mount pinning beyond the run lifetime is TT-1.

## Verification

See `verification.md` for the exact commands and outcomes.

## Acceptance

See `acceptance.md` for the user-visible behavior contract.
