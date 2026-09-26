# MEM-0A upstream pin and drift reconciliation Implementation Plan

> **For agentic workers:** Use superpowers:executing-plans for native execution. Steps use checkbox syntax.

**Goal:** Every upstream contract issue #33 cites is pinned to an exact revision, and documented drift between upstream docs and upstream code is reconciled.
**Architecture:** One research note; read-only upstream inspection; no Vivy code change.
**Tech Stack:** git, GitHub, rg.
**Spec:** [design](../../specs/2026-09-26-memory-providers-design.md) §Gaps, §Source baseline. State/dependencies: [index](index.md).

## Global Constraints

- Pin commits, not branches; record `sha`, `date`, `url` per upstream repo.
- Evidence = inspected file lines. Mark anything inferred, not verified.
- No code changes; deliverable is one note file.

## Task 1: Pin and inventory agent-diva BML

**Files:**
- Create: `docs/research/2026-09-26-memory-upstream-pin.md`

- [ ] **Step 1: Record pinned revision**

  Local clone `~/reference/agent-diva` HEAD `0fd005a1`. Record sha, remote
  `ProjectViVy/agent-diva`, date, and the branch (`main` — note `agent-diva-pro`
  divergence flagged by its CLAUDE.md).

- [ ] **Step 2: Inventory BML surface**

  Read `agent-diva-laputa/src/bml/mod.rs` (35 lines) and `memory_home.rs`
  (1119 lines). List exported types/functions: memory CRUD, FTS5 schema,
  scope model, provenance fields, revision/tombstone semantics. For each,
  record `file:line` evidence. Confirm or correct the issue claim that
  ordinary memory mutation is separate from persona governed-apply — find
  the call sites that prove it.

- [ ] **Step 3: Record the MemoryHome path drift**

  Issue says README describes profile-local `.laputa/` while code uses
  `config-dir/memory/memory.sqlite3`. Record the actual resolved path at
  `memory_home.rs` and mark which doc is stale.

## Task 2: Pin and inventory laputa

**Files:**
- Modify: `docs/research/2026-09-26-memory-upstream-pin.md`

- [ ] **Step 1: Clone and pin**

  `git clone https://github.com/ProjectViVy/laputa ~/reference/laputa`; record
  HEAD sha + date. If the clone fails, mark the pin as remote-URL + latest
  known sha from the issue's links and flag network as the blocker.

- [ ] **Step 2: Verify module map**

  Confirm `laputa/`, `mentle/`, `garden/` module layout and the
  `mempalace-go` naming in `mentle/README.md`. Record `garden/internal/recall/fast.go`
  dependence on Mentle facade types (issue claim) with line evidence.

- [ ] **Step 3: Re-check GovernedService.Mutate readiness**

  Re-inspect `laputa/governance/governed.go` at the pinned sha: does Mutate
  still write state before audit append and ignore append error? Is
  RollbackRef still a state hash without a recoverable snapshot? Record
  verbatim evidence; outcome feeds MEM-0D's readiness verdict.

- [ ] **Step 4: Cognitive partition ADR status**

  Check `docs/architecture/0002-*.md` and code for whether MEMRULES/WORLD
  semantics shipped; record target vs shipped state.

## Task 3: Pin third-party provider docs

- [ ] **Step 1:** Record pinned doc/commit refs for mem0 (`mem0ai/mem0`),
  memU developer contract (`NevaMind-AI/memU` `docs/developer.md`), and
  TencentDB Agent Memory (`TencentCloud/TencentDB-Agent-Memory`). Note the
  memU v1 constraints the issue lists (completed sessions in, one active
  developer run per workspace) with doc evidence.

## Task 4: Reconcile drift and commit

- [ ] **Step 1:** Close the note with a drift table: `claim → pinned evidence → status (confirmed/stale/changed)`.
- [ ] **Step 2:** Run `gofmt`-equivalent n/a; verify markdown renders, commit:

```bash
git add docs/research/2026-09-26-memory-upstream-pin.md
git commit -m "docs(memory): pin upstream revisions for issue #33 G0"
```
