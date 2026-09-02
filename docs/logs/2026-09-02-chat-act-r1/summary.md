# UI-CHAT-ACT R1 — kernel session/rewind (logical truncation markers)

## What changed

Implements R1 of `docs/architecture/JOURNAL-REWIND-AND-FORK.md`: the kernel
can void a session's visible history from a cutoff message onward without
deleting a single row.

- **Storage**: new `session_truncations` table (sqlite `migration020`,
  postgres schema; newest-row-wins index; session cascade delete wired into
  both backends' `DeleteSession`) and a `storage.TruncationStore` contract
  (`RecordSessionTruncation` / `LatestSessionTruncation`). Pure helper
  `storage.ApplySessionTruncation` implements the fold: cutoff is exclusive
  (messages strictly before the cutoff survive), `fork`-reason markers filter
  nothing, a stale cutoff fails open.
- **Runtime**: `Service.RewindSession` records the marker, journals a
  `session.truncated` event on a synthetic `tr_`-prefixed run (mirroring the
  compaction `cmp_` pattern), and returns the cutoff + remaining count.
  Guards: any non-terminal run on the session → `ErrSessionBusy`; cutoff not
  in the stored list → `ErrInvalidCutoff`; unwired store → `ErrRewindNotWired`.
  One shared filter `Service.effectiveSessionMessages` is applied at the three
  read views (model context via `runMessages`, `trajectory/session`, and the
  RPC `session/messages` handler). Run/log replay is deliberately NOT
  filtered — that axis stays a complete audit trail.
- **RPC**: `session/rewind {session_id, message_id}` →
  `{cutoff_message_id, remaining_count}`; error mapping per the
  `context/compact` precedent (InvalidParams / MethodNotFound / NotFound /
  Conflict). `session.rewind` added to the capability list.
- **App wiring**: `Truncations: backend` at both composition sites.

## Explicitly not done (R2)

- `session/fork` RPC (history copy to a child session).
- UI wiring: edit/rewind/fork buttons still disabled placeholders.
- The `edit` form (rewind + immediate `turn/start` with new text) is pure
  composition of shipped pieces and lands with the UI wiring.

## Design decisions honored

- Append-only Journal preserved: zero row deletion, filtering is read-time.
- Truncation fold composes with compaction fold (truncation first, then
  compaction), so compaction rows whose tail falls in the excluded region go
  inert naturally.
- Mount/approval governance untouched; busy gate reuses the recovery gate's
  definition of "active" (non-terminal run rows).
