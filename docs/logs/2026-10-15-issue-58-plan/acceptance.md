# How to review this planning delivery

Open the [index](../../superpowers/plans/issue-58/index.md), then the linked design.
Confirm the single owner, trusted sharing boundary, evidence-before-retirement
ordering, ordinary-session policy isolation, worker permissions, UI conflict/clear/
restore behavior and backend semantics match the intended product. D1 is withdrawn:
the confirmed LIFE singleton has no user-facing session model or Coding/LIFE toggle.
It retains governed coding/tool capability; avatar/name edits do not rekey/reset it.
Verify #58 consumes #66's trusted binding without implementing its full sidebar UI.
Do not ask for the confirmed product shape again; engineering choices remain proposed.

Each Story must name concrete existing/proposed files, producer/consumer contracts,
ordered steps, meaningful tests, evidence, rollback and readiness. The DAG must
match those contracts and one interface writer must own shared changes. Review
findings must be resolved or explicitly left blocking before execution.

This delivery succeeds as a plan only. It does not demonstrate long-term memory
quality, UI functionality, crash durability or installed behavior. S6 centralizes
those product gates on one candidate, and preserves a separate pending
Mentle-through-DIVA gate. Do not mark Issue #58 delivered from this package alone.

Rollback of this delivery is a documentation revert; no runtime state changed.
