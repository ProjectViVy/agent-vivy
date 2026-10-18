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

This is a bounded PR #82 repair. It does not integrate PR #79 with current main
or establish the final #58 release acceptance.
