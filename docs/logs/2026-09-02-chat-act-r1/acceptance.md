# Acceptance — UI-CHAT-ACT R1

How a human can tell R1 shipped:

1. **Store audit**: open the Journal (sqlite `data/vivy.db` in a daily
   install; this slice's own tests use throwaway DBs) after R2 ships and a
   rewind is performed — `SELECT * FROM session_truncations;` shows one row
   per rewind with the cutoff message id and `reason='rewind'`, and the
   `messages` table still contains every row, including the ones no longer
   shown.
2. **Audit trail**: the session's run log (`run/log` in the UI console, or
   the `run_events` table) contains a `session.truncated` event whose
   payload names the cutoff — nothing about the rewind is silent.
3. **RPC surface**: `initialize`/`capabilities` now lists `session.rewind`;
   calling `session/rewind` with a bad message id returns a JSON-RPC
   NotFound error, and calling it while the session has a running turn
   returns Conflict.

R1 itself has no button to press — the browser-visible acceptance (edit /
rewind actions in the chat toolbar actually truncating the visible history)
arrives with R2 and will get its own smoke record there.
