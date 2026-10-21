# D03/D08 transition fragments

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

Migration 037 drops the prompt payload digest and written-only Blob checksum.
`settle_baseline_runs.sql` disables only `ckpt-` pointers, preserving their
generations and unrelated private Blob owners. It expires pending questions
and commits one terminal per old nonterminal Run: active becomes failed,
accepted/queued becomes cancelled. Existing terminal history stays unchanged.
The D03 expiry fragment must execute first in the same transaction. D11's
Work/Plan and workflow settlement and D12's final metadata must join this
transaction when D14 assembles the finite artifact; these fragments alone
are not a publishable upgrade tool. No source database is modified.

D12 owns final metadata and D14 owns the finite conversion artifact and its
acceptance. `ApplyManifest` remains the existing explicit disposable-fixture
conversion seam; runtime `Apply` refuses old or unsupported stores before any
write. Accepted D03 target 036 is an explicitly recognized upgrade source;
unlisted historical 036–043 formats remain unsupported. Neither D03 nor D08
alone makes final upgrade publication possible.


Stable integration after D08: the owner-approved donor D06 allocation is now
paired038_generation_identity, removing generations.artifact_sha256. Stable
001–037 are unchanged. Exact accepted D03 036 and D08 037 source manifests are
recognized only for explicit copied upgrade; startup returns upgrade_required.
D06's earlier donor-only 037 target is historical test data, not an added source
format. Generation history/rollback checks are rerun against the combined target.
