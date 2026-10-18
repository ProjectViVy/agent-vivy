# Acceptance

For a new interaction, passing any non-empty `Disposition` to ordinary
`SealSubjectInteraction` returns `ErrSubjectInteractionConflict` and consumes no
history sequence. A healthy run therefore cannot be hidden by caller choice.

`SealSubjectExclusion` remains the trusted path: it accepts only evidence that
storage re-derives as permanently unresolvable, keeps the unresolved operation
recorded, and advances history once. Exact replay of that stored row through
either seal method returns the same interaction without duplicating history.
Excluded interactions are omitted from Recap; this says nothing about whether
an external side effect happened.
