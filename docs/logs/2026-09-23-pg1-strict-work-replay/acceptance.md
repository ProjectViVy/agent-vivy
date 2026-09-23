# Acceptance

Create a Goal and pause, block, or complete it. A later round-admission event must be rejected and must not increment the durable round count.

Replay an existing session one page at a time. Valid pages retain their stored WorkSeq values. If an earlier row has an unsupported payload version or a row carries a stale Goal reference, `ReadWork` and `ReplayWork` both fail instead of exposing a partial valid-looking stream. Existing request retry, stale expected version, and cross-session rules remain covered by domain and storage conformance tests.

For a Goal edit, submit the exact current Goal reference. The edit succeeds and the stored revision advances by one; a stale or cross-session reference is rejected. For replay, begin with an empty session `WorkState`, pass the returned state to the next page, and reject a mismatched cursor or sequence gap. A later corrupt record must not invalidate an earlier bounded page until reached. Goal recovery, Plan retrieval, and subscriptions use this same cursor contract.

On restart, an admission found in an early replay page does not grant Goal authority until recovery reaches the end of the stream. If a later page is corrupt, recovery returns no Goal reference; if all pages validate, it returns the admitted reference.
