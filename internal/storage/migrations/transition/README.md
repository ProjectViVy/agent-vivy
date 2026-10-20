# D03 transition fragments

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
acceptance. `ApplyManifest` enforces the same runtime format barrier as `Apply`.
Package-private test-only SQL helpers construct disposable business fixtures;
they provide no live conversion API. Both runtime entries refuse old or
unsupported stores before any write. D03 alone does not make final upgrade publication possible.
