# Baseline A re-anchor and retained-package epic split

Date: 2026-10-19
Branch: `feat/agent-loop-deletion` (from `202aac38`)

## What changed

- The owner selected baseline A: `202aac38` (2026-10-07), the last stable
  pi-parity closeout that passed benchmark runs. The 168 commits and 30
  first-parent merges added after it are unverified increments; they are not
  assumed present and are to be re-reviewed and re-introduced in gated batches.
- The issue-91 planning commits were replayed onto baseline A. Ten docs-only
  commits from `codex/agent-loop-deletion` were cherry-picked; `docs/TODO.md`
  was kept at its baseline-A form and the issue-91 tracking row was re-added as
  `ISSUE-91-DELETION`. No product code, generated file, schema or test changed.
- `D04` was narrowed to the inert loop-exp seam for nudge plus removal of the
  nudge FNV window and the consumer-owned result handoff. `D05` was narrowed to
  the warming/steering seam plus removal of warming's hash-only diagnostics and
  steering's custom hash binding/duplicate setup. Both now record an interim
  contract: the single core implementation keeps running the real behavior until
  the corresponding epic cuts over, and no dual path is permitted at any point.
- The three retained packages were split into independent epics: E-NUDGE,
  E-WARMING and E-STEERING, each with its own package, Story DAG and acceptance.
  C11 physical omission is claimed only by the epics; #91 does not claim it.
- Added the baseline-A re-anchor audit and updated the plan, the Story index and
  the TODO board to record the delivery split.

## Scope

Planning and documentation only: no runtime deletion, no package
implementation, no migration, no generated artifact, no product CI, no
real-token or real-path behavior, and no performance claim. The re-anchor audit
is a path-level comparison against this tree, not a behavior audit.

## Owner decisions recorded after the first push

See [owner-decisions.md](owner-decisions.md): the loop hard stop moves with
E-NUDGE, warming becomes opt-in (not selected by the default-family Recipes),
nudge is not installed for now (provisional, revisit later), D07 is retained as
the re-intake checkpoint for bundle/toolkit, and the post-baseline re-intake
remains a later gated track.

## Explicitly not done

- Re-introduction of the discarded 2026-10-08..10-18 work. That is a separate
  gated batch track sourced from `backup/main-pre-rollback-20261019`; each batch
  must pass the post-#91 contract (no custom hashes, one execution path, no
  second runner/queue/coordinator, CI and benchmark non-regression).
- Implementation of the base cut or any epic.
- GitHub epic issues and any push beyond publishing this planning branch.
- A claim that the 10-18 plan's audit evidence applies unchanged at baseline A;
  the re-anchor audit records every mismatch instead.
