# P2.1: Original-version cognitive state CAS

## Result

Implemented the C1 state transition repair on branch `feat/issue32-remediation`.
The task prevents stale cognitive attempts from overwriting accepted capture
watermarks or newer policy revisions.

## Changes

- `saveCognitiveState` now requires the version originally read and sends that
  exact version to `SnapshotStore.Put`.
- Pure state updates serialize through `cogStateMu`, reload the current
  snapshot, run an error-returning mutation, and commit with its read version.
- Policy CAS uses that same version and maps a storage version conflict to
  `ErrPolicyConflict`.
- Manual and automatic admissions serialize through `cogAttemptMu`. An
  admission commits against its original version. If a concurrent pure update
  wins first, the attempt rebases only its changed transition fields onto the
  latest snapshot; it does not repeat external reads or workflow effects.
- Added stale-version, concurrent capture/policy/settlement, and equal-base
  policy CAS regressions.

## Evidence

The three new regressions failed on the baseline for the intended reasons and
passed after the changes. The complete cognitive race test selection, focused
runtime vet, formatting, and whitespace checks passed. Aggregate `just ci` was
not available because this environment does not provide `just`; see
`verification.md`.

P2.1 is implemented. P2.2 intent persistence and crash reconciliation remain
the next planned cognitive task; this log does not claim product or release
acceptance.
