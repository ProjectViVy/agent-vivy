# Recap exclusion storage boundary

The peer repairs in PR #82 added a dedicated storage path that re-derives
whether terminal evidence is permanently unresolvable before sealing an
interaction as excluded. Review found that the ordinary
`SealSubjectInteraction` path still accepted a new caller-supplied `excluded`
disposition. A healthy terminal interaction could therefore bypass the
dedicated eligibility check and later read as an empty original.

New ordinary seals now reject caller-supplied dispositions before allocating a
history sequence. Existing exact idempotent replay remains valid, including
replay of an exclusion originally produced by `SealSubjectExclusion`.
That dedicated path remains the only way to create a new excluded row and still
re-derives irrecoverability transactionally. No evidence rows or operations are
deleted, and exclusion from Recap does not imply that external side effects did
not occur.

The repository CI also exposed that the peer's new `approvals.quick.otherSession`
translation lacked a cross-face classification. Since only the Web approval
card renders it, the i18n contract now marks it Web-specific.

The PR's native Windows owner lane previously stopped before running Garden
tests because the Laputa pin omitted the Windows `renameio.TempFile` shim.
Laputa PR #10 now contains the bounded Garden compatibility and Windows test
cleanup fixes; this candidate pins its exact head `ccf0e253bde3b9297c2f287cfe1a809813e9aabc`
in `laputa-source.lock.json`, all three module pseudo-versions, and `go.sum`.
The split Recap browser config now launches the checked-in Vite entry directly
from `ui/`, avoiding the broken pnpm shim path seen in its old remote run.

This is a bounded PR #82 repair. It does not integrate PR #79 with current main
or establish the final #58 release acceptance.
