# Baseline A re-anchor and epic split — acceptance

*Human view of what to check on this branch; it does not claim runtime behavior.*

1. The branch is `feat/agent-loop-deletion` and its first parent commit is
   `202aac38` (2026-10-07). Its only content changes are planning records and
   the issue-91 docs landing; no product code or generated artifact differs.
2. `docs/research/2026-10-19-baseline-a-reanchor.md` shows, per Story D00–D14,
   which 10-18 paths exist at baseline A and the disposition of every missing
   one; it also maps C1–C11 to the 10-07 tree.
3. `docs/plans/issue-91/index.md` states the baseline split: #91 is
   the base cut (hash retirement, core contraction, seam) and the three retained
   packages are delivered by E-NUDGE/E-WARMING/E-STEERING.
4. `D04` and `D05` are seam-only Stories: they no longer create
   `plugins/exp/*` or claim package parity; each links its epic and states the
   interim single-core behavior.
5. The three epic packages exist under `docs/plans/` with an index,
   four Stories each, explicit shared gates (T1-only factory ports, Eino
   capability check, import quarantine, physical omission, no hash, no second
   path) and their own acceptance.
6. `docs/TODO.md` opens with the `ISSUE-91-DELETION` row pointing at the plan,
   the Story package and the re-anchor audit; it also records that the
   post-baseline increments are a separate gated batch track.

The planning branch is published so an upstream agent can read the same
package; implementation evidence, CI, migrations and real-path behavior remain
unverified and are not implied by this planning delivery.
