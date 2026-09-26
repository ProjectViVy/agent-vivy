# MEM-0C host contracts — persona projection and session export Implementation Plan

> **For agentic workers:** Use superpowers:executing-plans for native execution. Steps use checkbox syntax.

**Goal:** Close GAP-A (persona projection) and GAP-B (completed-session export) with Host-owned contracts precise enough to review.
**Architecture:** One architecture doc; both contracts are Host-owned seams that keep Runtime sole owner of model input.
**Tech Stack:** Markdown; citations to `internal/runtime`, `internal/contexthost`, `sdk/port/*`.
**Spec:** [design](../../specs/2026-09-26-memory-providers-design.md) §Gaps GAP-A/GAP-B, REQ-MEM-9/10/11. Predecessor: MEM-0B vocabulary. State: [index](index.md).

## Global Constraints

- Do not invent undocumented public model hooks; do not overload the pre-tool
  middleware; preserve the Eino import quarantine.
- Providers never read raw Journal; session ≠ run completion.
- Persona projection is data until a Host stage promotes it — a `required`
  contextsource Candidate is not a system instruction.

## Task 1: GAP-A — persona projection contract

**Files:**
- Create: `docs/architecture/VIVY-MEMORY-HOST-CONTRACTS.md`

- [ ] **Step 1:** Survey where the system prompt/persona enters model input
  today: `internal/runtime/prompt.go`, `internal/runtime/prompts/`, and the
  mask/persona admission path from MASK-3 (`docs/superpowers/plans/masks/MASK-3.md`
  describes checkpoint-bound immutable instruction). Record the exact existing
  seam a versioned persona projection would attach to — prefer the existing
  instruction/persona admission over any new hook.

- [ ] **Step 2:** Specify the contract: projection content shape (versioned,
  explicitly selected, revision-pinned per session lifecycle); when a newly
  approved revision takes effect (new sessions only vs. defined boundary);
  resume semantics (committed view, no re-resolution); failure policy
  (required-identity failure rejects — never silently yields a different
  persona); audit hooks.

## Task 2: GAP-B — completed-session export contract

- [ ] **Step 1:** In the same doc, define the export trigger: Host-owned,
  fired on session completion (enumerate what "session complete" means in
  `internal/runtime`/`internal/storage` today — session close, not
  `run.completed`), producing a canonical session projection (ordered
  user/assistant turns + metadata), delivered through the observer receipt
  path or a defined sibling, with an adapter-side idempotency ledger.

- [ ] **Step 2:** Define provider opt-in: a capability declaration bit
  (`async-extraction`/`session-ingest`) and per-scope enablement; define what
  is excluded (raw Journal fields, secrets, tool payloads beyond allowed
  projection).

## Task 3: Commit

- [ ] **Step 1:** Commit `docs(memory): G0 host contracts for persona projection and session export`.
