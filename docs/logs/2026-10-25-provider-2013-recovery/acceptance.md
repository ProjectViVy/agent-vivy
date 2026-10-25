# Acceptance: how to tell it worked

Owner-run, live provider. No automated test in this delivery touches MiniMax.

## Recovered call

Repeat the MiniMax-M3.1-Flash child-subagent test (`just dev`, then the case
that failed with `SUBAGENT_OK`):

- The child completes. In the child run's journal the first model call shows
  `model.call.finished{status:"failed"}` with `invalid params, 400 (2013)`
  followed by one `provider.retry` (`reject_reason:
  "provider_2013_first_request"`), one `auto_retry.started` whose reason carries
  `code=2013` and a `fingerprint=…`, one `auto_retry.finished{success:true}`,
  and then a successful `model.call.finished`.
- The same session survives: the asker sees the child's answer, not a failed
  child.
- Any later `wire` summary for a text-only turn reports `content_forms` as
  `string` for the user and assistant rows instead of `array`.

## Honest failure

- A second rejection on the replayed call, a rejection on any input that
  carries an assistant or tool row, a named `2013` variant (tool order,
  invalid function arguments JSON, context window), or an input failing the
  pre-send invariants must **not** produce `provider_2013_first_request`.
  The run fails with the provider's error and the `wire` summary of the
  rejected body; nothing is retried.
- Only one retry is granted per run across all reasons: a run that already
  retried an overflow or a tool-order fault does not get a second grant, and
  the log line records the previous reason.

## Wire shape

In any captured request body (operator-gated dump, once
`WIRE-FULL-BODY-DUMP` exists):

- `content` on `user`/`assistant` rows is a string when the turn had only text;
- `tool_calls[].function.arguments` is compact JSON, `{}` when the recorded
  arguments were empty or unparseable (accompanied by one warning that names
  the call id and byte length, never the content);
- no `tool_calls` entry lacks a following `role:"tool"` row and no `tool` row
  answers a call that is absent.

## What a recovered call is not

It is not evidence that the `2013` cause is known. The payload was already
canonical in the failing reproductions; the retry removes the failed child, the
normalizer removes non-canonical shapes from every replay, and the content
bisect remains blocked on an operator-gated full-body dump.
