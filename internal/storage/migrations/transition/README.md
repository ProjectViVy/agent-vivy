# D03 and D06 transition fragments

These paired SQL resources are deliberately outside the runtime migration
embed glob. They are not a runner or a publishable upgrade tool.

The old-release normalizer first produces a verified baseline 035 copy from
the finite sources in D00 section 11.7. D03 migration 036 removes operation
digests and adds NULL original input and NULL classification to every existing
operation, preserving its state, owner, result/failure and timestamps. Neither
the migration nor the approval fragment reconstructs missing authority.

On the target copy only, D14 binds the conversion timestamp and executes the
approval fragment with D08 checkpoint disabling, interaction settlement and
exactly one Run terminal in one transaction. Pending and possibly unconsumed
approved authority on nonterminal Runs expires with
`upgrade_recovery_required`. Terminal history is preserved. Expiry does not
assert that an operation had no effect; claimed operations remain fenced.

D12 owns final metadata and D14 owns the finite conversion artifact and its
acceptance. `ApplyManifest` remains the existing explicit disposable-fixture
conversion seam; runtime `Apply` refuses old or unsupported stores before any
write. D03 alone does not make final upgrade publication possible.

## D06 Generation identity final target

Paired migration 037 removes the frozen core `generations.artifact_sha256`
column while preserving IDs, parent/source/Recipe/phase/timestamp values and
all evaluation, promotion and event history. This does not convert or touch
the independent Studio lifecycle ledger; that owner supplies its own fragment.

Runtime preflight still refuses supported old sources before writes. The
explicit finite path remains old normalizer -> baseline035 copied target ->
`ApplyManifest` through 036 and 037 -> verified target reopen. Source heads036
from interim development are unlisted formats, not an automatic upgrade route.
A marker alone does not admit a target that retains the retired Generation
column or lacks required final columns. D14 still owns the complete published
upgrade artifact and the final configuration switch. Neither037 nor these
fixtures authorize an in-place production migration.
