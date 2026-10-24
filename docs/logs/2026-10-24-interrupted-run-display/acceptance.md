# Interrupted run display — acceptance

## What a person should see

1. Open a cancelled run in the chat (on the dev home, session
   `sess_40d6e291f2e0cb3f`, or `run_d7da382bbc231d8c` through Run Inspector).
   The final thought shows its text with a muted **已中断** chip next to
   `思考过程`; it does not animate as if still running. Text/reasoning blocks
   that completed before the cancel look unchanged (no chip).
2. Stop a long answer while it is streaming. The partial assistant text stays
   visible, marked **已中断** before the timestamp, and no second bubble appears
   after it.
3. Open a completed run (e.g. session `sess_fed3d415787a9794`). No **已中断**
   appears anywhere.
4. Unfinished tool calls in a stopped run still read **已停止**, as before.
5. Copy on an interrupted bubble still copies its partial text; rewind/fork
   buttons are still present (see the known limitation in `summary.md`).

## Bilingual check

The chip follows the UI locale: `已中断` in `zh`, `Interrupted` in `en`.

## What did not change

The message store, Journal, migration head, and `session/messages` payloads are
unchanged. A cancelled run still has no persisted assistant message for its
partial output; the marker exists only in the browser fold.
