# Acceptance

Concurrent policy, capture and wake activity preserves each accepted input and admitted workflow identity. One policy base revision has exactly one accepted writer; its competitor gets ErrPolicyConflict. Repeated simultaneous manual and automatic wakes do not create overlapping windows.

This repairs in-process Service state concurrency. The per-key SQLite CAS remains authoritative against stale versions. It is not a claim of multiple writable host processes sharing a profile, nor complete S06/S10 acceptance. Receipt recovery, lifecycle and crash cases remain open local work.
