# MEM-0E tracker and architecture doc update Implementation Plan

> **For agentic workers:** Use superpowers:executing-plans for native execution. Steps use checkbox syntax.

**Goal:** Repo trackers and SCX architecture docs reflect the scheduled MEMORY track and cite the landed G0 contracts.
**Architecture:** Small edits to existing authoritative docs; no duplicates.
**Tech Stack:** Markdown.
**Spec:** [design](../../specs/2026-09-26-memory-providers-design.md). Predecessor: MEM-0A–0D (cites real doc paths). State: [index](index.md).

## Global Constraints

- `AGENTS.md` communication rule: docs in English.
- Replace, don't append: MEM-1 tracker entry gets rewritten to point at this
  package; do not leave a second stale memory entry.

## Task 1: `docs/TODO.md`

- [ ] **Step 1:** Replace/expand the deferred MEM-1 row into stage rows
  MEM-0..MEM-5 with status pointers to `docs/superpowers/plans/memory/index.md`
  and the landed contract doc paths.
- [ ] **Step 2:** Update the UI-EVO row note (it awaits MEM-1): point it at the
  memory package and the new profile doc for its future capability switch.

## Task 2: `docs/DEFER.MD` + `docs/research/OPEN-ITEMS.md`

- [ ] **Step 1:** Rewrite the MEM-1 DEFER row: memory is no longer fully
  deferred — G0 contract freeze is scheduled; keep AutoDream/Evolution/RAG
  deferred under their own IDs if they share the row. OPEN-ITEMS row updated to
  match.

## Task 3: SCX cross-refs

- [ ] **Step 1:** In `docs/architecture/SCX-PLUGIN-INTEGRATION.md` memory rows,
  add one-line pointers to `VIVY-MEMORY-PROFILE.md` and
  `VIVY-MEMORY-HOST-CONTRACTS.md` as the frozen contracts. In
  `SCX-ARCHITECTURE-DESIGN.md` §C (memory return/dedup), add a pointer to the
  profile. One line each — no restructuring.

## Task 4: Iteration log + commit

- [ ] **Step 1:** File `docs/logs/2026-09-26-memory-g0-contracts/` with
  `summary.md`, `verification.md`, `acceptance.md` covering the whole G0 story
  set (per repo convention).
- [ ] **Step 2:** Commit `docs(memory): schedule memory track, update trackers for G0`.
