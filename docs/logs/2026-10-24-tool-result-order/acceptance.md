# Acceptance

## Reproduce the fixed behaviour

1. Open the split pair at `http://127.0.0.1:3015`.
2. Resume a session that previously failed with
   `400 ... tool call result does not follow tool call (2013)`, or create a turn
   that mixes an approval-gated `bash` call with an `agent` call in one message.
3. Send the next message (or retry the failed run).

Expected: the turn proceeds normally. The model sees the `bash` result next to
its call and the `agent` result next to its call, in request order, no matter
which finished first.

Expected in the Journal: `tool.finished` events may still appear in completion
order — that is by design. Only the replayed model feed is ordered.

## Regression checks

- A mid-run steer still appears after the tool group, not inside it.
- A session with an orphan tool result still opens (the orphan is emitted, not
  dropped).
- Sessions without any tool calls are unchanged.

## What this does not claim

- A genuinely malformed provider request still fails; there is no retry on
  provider 400s.
- The intermittent child first-call 400 remains outside this change.
