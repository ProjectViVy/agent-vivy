# A2A-02 acceptance notes

Story outcome (E1 / R2, R5): atomic native admission landed.

- One commit transaction carries scope lock, durable receipt, candidate
  session, ownership row, message, run, prompt snapshot and both journal
  events (run.started seq=1, channel.task_admitted seq=2) on sqlite and
  postgres; identical concurrent retries resolve to exactly one accepted
  transaction through the receipt inside the lock scope.
- Retry ordering: receipt consulted before busy gates (projectionMu
  pre-check + in-transaction recheck); semantic map end to end —
  foreign identity ErrNotFound, same key different hash ErrConflict,
  busy context ErrWorkRunConflict, identical retry original receipt.
- Deletion writes owner and receipt tombstones inside the existing
  native delete transaction; tombstoned keys revive with fresh session
  identity and dead address space is never resurrected; live receipts
  pointing at missing runs report corruption.
- Listing returns only non-tombstoned owned primary submissions with a
  keyset continuation flag; no status or output cache added.
- Startup sweep reaps provisional workspaces empty+unlisted+older than
  the admission grace window, wired into App.Run/StartEmbeddedServices.

`just ci` red only via the documented main regression
`TestClientAgainstRealVivyCode` (PR #37 pending merge; CN-21 cleared
by PR #38 merging into this branch). Everything else green: all
storage/runtime/matrix/payload tests on both engines, go-host pack
suite, conformance reproduction, ui 598 tests, i18n, fmt, lockfile.

Open owner items carried forward:

1. Laputa pin bump for `personactx.Store.DiscardSession` /
   `ListFrozenSessions` — the `DiscardFrozenSession` seam and the
   workspace sweep are in place; frozen-persona discard stays optional
   and logs until the pin lands.
2. Applying the amended reconnect criterion text to issue #2 itself
   (draft in design section 13; not yet posted).
3. PR #37 merge so `just ci` returns green on this branch.
4. Blueprint suggestion (postgres init step) still awaiting approval.
