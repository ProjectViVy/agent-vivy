# Reflection and reusable process restart

The S03 fixture now runs a deterministic reflection provider through the real HTTP/model path. Its replies derive only from the actual source envelopes in recorded requests; random facts never enter a responder closure. A reflected observation joins public effect receipts and the processed source watermark to the actual canonical row.

Restart closes the current actual App and starts an owned OS child using the same profile and generated composition. Fixture RPC, observations and request counters forward to that child. PID checks, unchanged canonical identity/content and fresh counters prove process continuity. Only the owned child can be killed on timeout; no broad process cleanup is used.

Lifecycle regressions found two fixture defects: repeated Close consumed the process-exit notification twice and timed out; actions after Restart used the old peer. Close now remembers its first outcome, and actions use the fixture's active RPC path.

The preceding cognitive stage-budget product fix is 9d35e50c. This increment changes test infrastructure only. Recall and the remaining S04–S11 chain work remain locally executable work, not external deferrals. Formal Story acceptance, final source sealing and full CI remain pending for the new phase.
