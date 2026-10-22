# Stable source integration

The owner authorized pushing the integrated D-series candidate to `stable`.
The verified executable source is `8b00475b`, with Laputa
`9acc999301d5d575da2cea902a1c6d213763f381`, INOFY
`6f1397e417fc524ad266634247dd632e2ed93ec0`, and Eino v0.9.21.
This following integration record changes documentation only.

The integration includes D09/D12, upstream D10/D11 adaptation, D14 owner
corrections, the user-defined provider repair and the real-model/tool benchmark.
Pre-push Linux verification additionally exposed a cancelled SQLite admission
error losing its context cause. That narrow correction has deterministic
RED/GREEN, independent source review, native checks and renewed Linux gates.
The initial failure remains recorded rather than dismissed after a retry.

Integration uses a normal fast-forward from the clean local stable checkout.
The remote was observed at `fde3c1f96f086507a35e52689e58fc6e4db981b4`,
an ancestor of the candidate. No history rewrite is needed. Commits and the
push identity belong to the human contributor `mastwet`. The running disposable
manual-test backend is not restarted or replaced by the Git operation.

The sixteen baseline correctness repair intake items and two CI intake items
remain open in `docs/TODO.md`. This does not mark the remaining human Tiny Face
checklist or issue91/D14 complete. The read/list benchmark retains its actual
executable source397b6f17; it is not relabeled as a run of the SQLite correction.
The finite conversion artifact retains its original pins and is not republished.

This is source integration only: no product release, tenant upgrade, production
data access or live-key read-back. Separate release notes are omitted because
no release artifact is shipped here.
