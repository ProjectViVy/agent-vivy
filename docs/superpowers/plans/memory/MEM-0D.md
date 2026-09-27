# MEM-0D mutation authority and readiness revalidation Implementation Plan

> **For agentic workers:** Use superpowers:executing-plans for native execution. Steps use checkbox syntax.

**Goal:** Resolve every mutation route under current Grant ceilings, and revalidate the two external readiness claims (Eino fit, Laputa governance) against pinned revisions.
**Architecture:** One research/decision note; conclusions feed MEM-0E tracker update and unblock MEM-1.
**Tech Stack:** Markdown; SDK + go.mod inspection.
**Spec:** [design](../../specs/2026-09-26-memory-providers-design.md) §Gaps GAP-C/GAP-D. Predecessor: MEM-0A (pinned upstream). State: [index](index.md).

## Global Constraints

- Every mutation route must name an existing authorized Host operation or an
  external service route; otherwise record the smallest reviewed capability gap.
- BML first-party integration must respect T1 ownership boundaries (issue rule).
- Verdicts are binary: `fit` / `gap:<name>` / `blocked:<reason>`.

## Task 1: Mutation authority matrix (GAP-C)

**Files:**
- Create: `docs/research/2026-09-26-memory-g0-readiness.md`

- [ ] **Step 1:** Enumerate the closed mutation set from the profile:
  remember, correct, forget, configure, plus provider-internal maintenance.

- [ ] **Step 2:** For each, against `sdk/port/controlaction` `Host`
  (`Grant`, `Secret`, `Settings`, `StartRun`, `InvokeTool`) and the Grant
  catalog in `sdk/module/grant.go`, assign the route: e.g. BML local writes
  need a scoped `fs.write` Grant — verify that Grant exists and what its
  ceiling is; remote writes need `net.client`/`rpc.client` + `secret.read`.
  Record any missing grant as a named gap, not a workaround.

- [ ] **Step 3:** Decide T1 vs T2 for the BML module: first-party repo module
  vs plugin; record the ownership-boundary consequence per the issue rule.

## Task 2: Eino/EinoExt revalidation

- [ ] **Step 1:** Read pinned `go.mod` Eino/EinoExt versions (issue cites
  v0.9.13 — verify). Inspect the module cache for `components/retriever`,
  `components/indexer`, task/middleware/transport surfaces relevant to
  recall and extraction orchestration.

- [ ] **Step 2:** Record per surface: reused API, missing semantics, and the
  removal/migration boundary for any approved custom seam. Deferred
  capabilities stay deferred — no parallel implementation.

## Task 3: Laputa readiness verdict (GAP-D)

- [ ] **Step 1:** From MEM-0A's pinned inspection of
  `laputa/governance/governed.go`, write the readiness verdict covering:
  atomicity (state write vs audit append), recoverability of RollbackRef,
  durable audit, authenticated actor mapping, idempotency.
- [ ] **Step 2:** Verdict `fit` / `gap:<fix required upstream>` / `blocked`,
  with the upstream PR/file needed when not `fit`.

## Task 4: Commit

- [ ] Commit `docs(memory): G0 mutation authority and readiness revalidation`.
