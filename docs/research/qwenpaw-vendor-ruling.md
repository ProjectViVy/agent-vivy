# SR-4 Ruling — Keep QwenPaw as an external reference; do not vendor

> **Date:** 2026-09-02
> **Closes:** TODO §0.1 row SR-4 (vendor vs external); REFERENCE-INDEX RI-OQ-5
> **Constraint:** P2-3 (fsjournal probe specification) remains DEFERRED; this ruling does not change its status.

## 1. Question

QwenPaw (`agentscope-ai/QwenPaw`, Python, Apache-2.0) underwent P9-style verification on 2026-08-07 (the two strategies of Scroll=SQLite authority and Creator Runtime=SQLite-free, plus the implementation details of atomic_store/jsonl_store/locking/session_store; see `REFERENCE-INDEX.md` §3.17); it was cloned to `/tmp/QwenPaw` at the time and not vendored. RI-OQ-5 asks: should it be vendored into `.workspace/qwenpaw` in the future?

## 2. Ruling: **stay external (external-by-URL); do not vendor**

Reasons (in descending weight):

1. **No verified use case.** QwenPaw's only value to Vivy is a **design reference** for the V1+ filesystem journal backend probe (atomic publication / JSONL envelope / cross-process locking / crash-tail recovery). The probe's P2-3 specification row is DEFERRED, as is its prerequisite MEM-1 family. The recommendation in REFERENCE-INDEX §3.17 was already to "vendor only once there is a verified use case"; that condition remains unmet.
2. **Apache-2.0 means it can be retrieved at any time.** There is no licensing reason to "hold on to it now"—the upstream URL is a stable retrieval channel, and the license permits future probe projects to re-clone, copy, and adapt it when needed, with no time-limit risk.
3. **Disk hygiene and freshness of the source of truth.** `.workspace/` is a reference area, not an archive: the claude-code, rig, and other trees were cleaned after the August verification (confirmed on site on 2026-09-02). Vendoring a 160,000-line Python tree with no consumer would create a second stale source of truth of the form "the index says it is present, but it is absent from disk."
4. **The verification record is preserved.** All design-relevant conclusions (including specific file line numbers) are in `REFERENCE-INDEX.md` §3.17; when the probe is activated, it can simply be re-cloned and re-verified using the §3.17 "Open follow-up" process (the Python upstream is still actively evolving, so re-verification is more reliable than trusting an old copy).

## 3. Actions

- Do not create `.workspace/qwenpaw`.
- Mark the REFERENCE-INDEX §3.17 open follow-up (RI-OQ-5) RESOLVED, pointing to this document.
- TODO §0.1: mark SR-4 DONE (this ruling); add the note "SR-4 ruled external retrieval" to the P2-3 row, with status unchanged (DEFERRED).

## 4. When to reverse the ruling

Re-evaluate if any of the following occurs (following the capability re-entry process in ASSEMBLY-OPTIONS §6: proposal before work):

- the fsjournal probe (P2-3) is approved and enters implementation;
- a capability proposal explicitly requires implementation-level QwenPaw reference beyond the pattern-level conclusions preserved in §3.17.
