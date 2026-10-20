# D06 Studio identity and finite ledger transition

The independent Studio lifecycle ledger now stores Generation identity and
actual artifact SourceRef without computing or requiring artifact hashes.
Install and install inspection retain manifest identity, executable presence,
release/evaluation/actor paths and rollback behavior. An explicitly supplied
evaluation must belong to the Generation being released.

Fresh ledgers receive an explicit final format marker. Read-only schema guards
refuse legacy and unfamiliar shapes before CREATE or writes. The finite
`internal/transition/studioledger` artifact converts a quiescent disposable
source into a new owned target, preserves lifecycle history and event allocator
state, maps actual custom-output binary paths, copies rollback snapshots, and
publishes only the verified complete home. Source ledgers stay untouched.

Species Inspect projects Generation identity through `InspectManifestIdentity`
and drops artifact/UI/policy hash fields. The actual pinned Studio workbench
consumers were updated in local Studio commit `abc3ba4`.

The shared core `domain.Generation.ArtifactSHA256`, frozen species generation
writes and paired core SQL formats remain owned by the sequential D03/final
core-format integration. Removing their validation now would permit dummy
empty-hash writes into unchanged SQL, so that behavior is deliberately retained.
No tenant Journal, install, release or production store was accessed.

D14 must archive/build the finite upgrade artifact from its recorded source
commit, integrate the other owners' fragments and remove transition code from
the final release tree. Those integration and release gates remain open.
