# Acceptance

Create a Goal and pause, block, or complete it. A later round-admission event must be rejected and must not increment the durable round count.

Replay an existing session one page at a time. Valid pages retain their stored WorkSeq values. If an earlier row has an unsupported payload version or a row carries a stale Goal reference, `ReadWork` and `ReplayWork` both fail instead of exposing a partial valid-looking stream. Existing request retry, stale expected version, and cross-session rules remain covered by domain and storage conformance tests.
