# A2A-00 — Evidence gates and G0 adoption

Executed the three tasks of [A2A-00](../../superpowers/plans/2026-10-07-a2a-server/A2A-00.md)
on branch `A2A` after merging main at `b4ed6fe6`.

- **A2A-00.1** — built the reproducible official-client probe at
  `sdk/testdata/a2a-probe` (nested module, cannot ship): 26 checks green
  against pinned `a2a-go/v2 v2.6.0`. Report:
  `docs/research/2026-10-07-a2a-sdk-probe.md`. Commit `4550625d`.
- **A2A-00.2** — traced candidate-session write attribution through
  runtime/storage/pinned laputa; froze the contract into design 6.2 and
  A2A-02. Report: `docs/research/2026-10-07-a2a-native-preparation.md`.
  Negative finding: pinned laputa `personactx.Store` has no delete API —
  the frozen contract requires `DiscardSession`/`ListFrozenSessions`
  (laputa pin bump, owner-review item). Commit `c34a3be9`.
- **A2A-00.3** — owner adopted G0 on 2026-10-07: option A snapshot
  convergence (issue #2 reconnect criterion amended per design 13),
  remote ordinary answers included, minimum deployment
  (loopback + reverse proxy; multi-principal and Host TLS removed),
  opt-in Generation membership per issue #2. Reconciled design 1.1/8/10/
  13/14, `CH-C9`, `VIVY-CHANNEL-PACK`, `index.md` and `A2A-05`. A2A-R1 is
  unselected, not deleted.

No product code, dependency, migration, Recipe or listener was changed.
Functional Stories stay blocked on G1 scheduling.
