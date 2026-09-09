# Summary — SR-4: keep QwenPaw external; do not vendor

## What changed

- `docs/research/qwenpaw-vendor-ruling.md` (new): rules that QwenPaw stays
  external-by-URL. Reasons: no verified use case (fsjournal probe P2-3 DEFERRED), Apache-2.0
  is always available, and this avoids the stale source of truth where "the index claims it
  is present / the disk does not"; the §3.17 verification conclusion is preserved, and
  revalidation is preferable to trusting an old copy. Includes reversal conditions (a probe
  or proposal needing implementation-level reference).
- `docs/research/REFERENCE-INDEX.md`: marks the §3.17 open follow-up (RI-OQ-5) RESOLVED and
  adds a line in §6.
- `docs/research/OPEN-ITEMS.md` / `docs/TODO.md`: moves SR-4 to DONE; adds the ruling note
  to P2-3, keeps its status DEFERRED, and records it in TODO §10.

## What was explicitly not done

- Do not create `.workspace/qwenpaw`; do not run the fsjournal probe (P2-3 remains DEFERRED,
  and prerequisite MEM-1 is also DEFERRED).
- The decision to halt the entire channel family is unaffected.

## Scope

Docs-only; zero code.
