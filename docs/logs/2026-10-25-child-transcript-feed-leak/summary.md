# Child transcript leak into the parent model feed

Date: 2026-10-25
Scope: `internal/runtime` only. No provider, schema or UI change.

## Symptom

After a session that spawned six `agent` sub-agent children, the chat input
context ring reported **59%** for the next turn. The parent run itself never
exceeded **18,788 prompt tokens** (last of twelve calls in
`run_b67af7138e043698`).

## Root cause

Child runs share the parent session id, and the message projection writes every
run's transcript into that session store (`reconcileSessionMessageProjection`
iterates `ListRunsBySession`). Both feed consumers then read the whole session
without a lineage filter:

- `runMessagesForRunWithCollaboration` fed `ListMessages(sessionID)` to
  `buildRunContext`.
- `ContextStatus` folded and measured the same rows for the meter.

Measured session `sess_a3e51e9c26f6d4a4`: 206 rows / 280,905 bytes, of which
164 rows / 275,289 bytes belong to child runs (94 of them `tool` rows,
277,181 bytes). The meter divided ~72k estimated tokens by the 128k fallback
window, so the ring showed 56–59% while the parent's own transcript was 42 rows
/ 5,616 bytes.

The child's answer already reaches the parent as the `agent` tool result, so
the transcript was being counted twice, and on the next turn it would also have
consumed the parent's whole history budget (`runtime.max_context_bytes` defaults
to 256 KiB).

## Change

`Service.parentFeedMessages` drops rows whose run is a child run before the
feed is assembled. It fails open (keeps every row) when the run listing fails
or a row has no run id, so legacy projections never disappear.

Applied at both consumers:

- run feed assembly (`runMessagesForRunWithCollaboration`);
- `ContextStatus` (`feed_messages` / `feed_tokens`, and the fold input), while
  `total_messages` still reports the full stored transcript.

Child rows stay in the store: the UI transcript behind the `agent` tool card is
unchanged.

## Not done

- Child transcripts are still projected into the parent session store; only the
  model feed and its meter ignore them.
- No change to cancellation, mailbox delivery, or the `agent` tool result.
