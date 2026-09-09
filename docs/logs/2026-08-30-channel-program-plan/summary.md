# 2026-08-30 — Super Channel program plan

## Goal and background

Contract C0 was adopted. Per the project manager, split the implementation into
`docs/TODO.md`: claimable WBS items, a milestone calendar, a dependency activity graph,
and a single-lane Gantt.

## Changes

- `docs/TODO.md` §0.1: mark CH-A / CH-B SUPERSEDED; add CH-C1..C9;
  merge `UI-CHANNELS-BE` into CH-C5.
- `docs/TODO.md` §0.2: milestones M-CH0..M-CH5, person-days, critical path, Mermaid
  activity graph, and Gantt.
- `VIVY-CHANNEL-PACK.md` §20: the authoritative schedule points to §0.2.

Locked schedule:

| Milestone | Plan | Buffer |
|---|---|---|
| M-CH1 foundation (C1–C3) | 2026-09-08 | 2026-09-10 |
| M-CH2 first ear (telegram + Settings page) | 2026-09-14 | 2026-09-16 |
| M-CH4 iteration close (+discord) | 2026-09-24 | 2026-09-30 |

Next: **CH-C1**. C8 / C9 / WeCom are not part of this iteration.

## Explicitly not done

- No kernel / SDK / plugin code was implemented.
- The CH-C1 worktree was not created.
