# MEM-1 BML vertical slice — BLOCKED plan

> **Status: Blocked.** Execution requires MEM-0B (profile), MEM-0C (host
> contracts), MEM-0D (mutation authority + T1/T2 decision) outputs. This file
> records the shape and the exact decisions it waits on; do not implement from
> it yet.

**Goal:** First-party BML memory backend: bounded recall, explicit CRUD,
scoped isolation, inspectable status — the issue's G1.
**Spec:** [design](../../specs/2026-09-26-memory-providers-design.md). State:
[index](index.md).

## What this Story delivers (outline only)

- New repository Module (proposed `vivy/memory-bml`, own `plugins/` or
  `internal/modules/` home per MEM-0D's T1/T2 verdict) porting Diva BML storage
  semantics: typed SQLite + FTS5, scoped CRUD, provenance, revisions,
  tombstones. Explicitly NOT ported: Diva runtime loop and prompt assembly.
- Providers: `contextsource.Provider` (+ `Resolver` for evidence reads),
  `observer.ReceiptRunProvider` for committed-experience ingestion,
  `controlaction.Provider` set for remember/correct/forget/configure,
  `status.Provider` for health/coverage.
- Conformance evidence aligned to the SCX fixture pattern (fixtures proving:
  no pre-commit delivery, one logical update after ambiguous ack, scope
  isolation across tenant/workspace/session, bounded recall, explicit
  unsupported-op errors).
- `plugins/vivy-memory` UI rewired off `getDemoMemories()` stubs to the real
  backend via the defined read path — only if the UI seam lands cleanly;
  otherwise the demo banner stays and UI rewire moves to a follow-up Story.

## Blocking decisions (must be resolved by G0 docs)

1. **Mutation route (MEM-0D):** which authorized Host operation or scoped
   Grant carries BML local writes; whether `vivy/memory-bml` is T1 repo module
   or T2 plugin, and the ownership boundary each choice implies.
2. **Profile fields (MEM-0B):** exact envelope/capability/error names the
   adapter must emit.
3. **Scope identity (MEM-0B/0C):** how user/persona identity reaches the
   adapter (Host-assigned fields; extension of `contextsource.Scope` vs.
   a parallel identity channel).
4. **Storage home (MEM-0D):** sanctioned path for the BML SQLite file under
   scoped fs authority — not `data/vivy.db`, not the Journal.

## Known non-goals

Diva Runtime/prompt orchestration; vector/graph features; any persona
governance (MEM-3); Garden (MEM-4); writes to Vivy core storage.
