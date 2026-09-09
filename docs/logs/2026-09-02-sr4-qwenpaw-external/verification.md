# Verification — SR-4 QwenPaw vendor ruling

Date: 2026-09-02.

```text
ls .workspace/
  → no qwenpaw directory (the ruling is implemented as "zero disk action"; no vendor needed)

just ci   (combined docs gate with today's P3 license review / P2-1 inventory slices)
  → CI-EXIT:0
```

Fact-source check: `REFERENCE-INDEX.md` §3.17 (the 2026-08-07 P9-style verification record:
Apache-2.0, the Scroll/Creator dual strategy, atomic_store/jsonl_store/locking/session_store
details, and shadow-Git checkpoint not being Eino) — every technical conclusion cited by the
ruling comes from that existing verification; no new upstream fetch was performed.
