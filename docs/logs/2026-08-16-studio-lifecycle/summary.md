# Studio Lifecycle Closure Summary

Date: 2026-08-16
Status: complete

## Outcome

ST-5 / ST-7 / ST-8 are done, developed inside Vivy Studio (author path is
this Studio session). The Studio now owns the full lifecycle — pack, eval,
release, install, rollback — through a dedicated Studio binary
(`vivy-studio.exe`), a Studio-owned ledger (`data/studio-home/studio.db`),
and a prefab skill (`vivy-studio-lifecycle`). The live species process
participates in none of it.

## Delivered

- `internal/domain`: Studio lifecycle objects per `VIVY-STUDIO.md` §8 —
  `Worktree`, `Release`, `Install`, phases `eval_pending`/`released`, event
  types `release.accepted` / `install.recorded` / `install.rolled_back` /
  `worktree.pinned`.
- `internal/eval`: extracted a store-agnostic launcher (`Launch`,
  `CandidateExecutable`) so both the frozen species-side runner and the
  Studio core spawn candidates through the same air-gap isolator.
- `internal/studiocore`: Studio-owned SQLite ledger (Worktree / Generation
  / EvalRun / Release / Install + studio_events) at
  `data/studio-home/studio.db` and the lifecycle service: `Pack` (execs
  `vivy-sdk`), `Eval` (Studio spawns the candidate EXE itself), `Release`
  (human-only), `Install` (writes the daily location + manifest),
  `Rollback` (restores the previous release from a Studio snapshot).
- `cmd/vivy-studio`: the Studio lifecycle CLI (`workspace`, `pack`, `eval`,
  `release`, `reject`, `install`, `rollback`, `inspect`, `list`).
- Tests: ledger lifecycle, human-gate on release, candidate spawned by the
  Studio (real `cmd/vivy` build), install→rollback round trip, tenant
  Journal untouched, target guard.
- `.agents/skills/vivy-studio-lifecycle/SKILL.md`: the Studio-side
  workflow for pack → eval → release → install → rollback.
- `justfile` `studio` recipe builds `vivy-studio.exe`.

## Explicitly deferred

Frozen eval suite (S9) beyond the air-gap probe; automatic release rules
(a later decision); Studio UI for publish beyond the CLI + skill gate.
