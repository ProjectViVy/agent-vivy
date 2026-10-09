# P4.1b cognitive bundle cleanup

## Outcome

Implemented C7 on `feat/issue32-remediation`, source commit
`9f6595bf` (`fix(app): release cognitive ownership on failed composition`).

After the selected cognitive factory returns a bundle, `NewWithAssembly`
registers one cleanup owner before any later composition stage can fail. The
guard joins `Bundle.Close` errors with the original initialization error and is
disabled only after the completed App takes ownership. Branch-local closes
were removed to prevent skipped cleanup and duplicate Close calls.

The storage backend now has a failure guard at the same composition boundary.
On failure after factory construction, observer subscriptions close first,
then the cognitive bundle, then the backend. Successful composition transfers
both owners to App.Close. A narrow storage-open seam supports a backend fixture
that omits only RunAdmissionStore while satisfying the earlier contracts.

## Evidence

- Fault injection covers observer-host construction, primary admission,
  binding resolution, runtime control attachment, and cognitive action
  dispatcher validation.
- Each failure is retried against the same isolated storage root with a fresh
  bundle; each bundle closes once and the backend can be opened again.
- Cleanup sentinel and original composition error both remain discoverable
  with `errors.Is`.
- Successful ownership transfer does not close during construction; repeated
  App.Close closes the bundle exactly once.
- App package tests pass; see [verification](verification.md).
