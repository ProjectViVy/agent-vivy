# PG-5: Session notifications and GUI work controls Implementation Plan

> **For agentic workers:** Use superpowers:executing-plans or superpowers:subagent-driven-development only after design review, prerequisite evidence and explicit implementation authorization. Steps use checkboxes.

**Goal:** Users can inspect and control Plan/Goal, reconnect and see later automatic runs.
**Architecture:** Follow PG-D1 shared ownership and transaction contracts. Keep this slice within existing Service / Journal / Policy ownership.
**Tech Stack:** Go, pinned Eino v0.9.13, SQLite/PostgreSQL, React/TypeScript where applicable.
**Spec:** [PG-D1](../../architecture/PLAN-GOAL-PREDESIGN.md).
**Baseline:** a8d361b0244a1c40be513622bbdaebb5c9d40014.
**Epic:** Composition. **Requirements:** R8, R2, R5.
**Status and predecessors:** [Authoritative index](README.md). Do not infer Ready from this file.

## Global Constraints

- Soft Plan cannot expand independently configured permissions.
- Armed Goal and effective Plan cannot coexist.
- One Service / Journal / Policy path; Eino imports remain in runtime/provider.
- No code, dependency, database or external tracker mutation is authorized merely by this pre-design.
- Preserve user-owned work. Rebase existing file locations and migration versions before execution.
- Test commands below are future instructions; none are claimed passed.

## Contracts and prerequisites

Consumes: PG-4 RPC methods and PG-3 activation/current-run view; existing single UI store and WebSocket transport.

Produces: WorkView-backed controls, exact plan review, session replay/live subscription and automatic-run discovery.

This pre-design is not a claim that proposed interfaces exist. PG-0 must settle shared blockers before production code is added. Use the index for prerequisite evidence; revise dependent plans when PG-D2 changes any shared signature.

## File ownership

Existing read/modify scope:
- `ui/src/lib/api.ts`
- `ui/src/lib/store.ts`
- `ui/src/lib/rpc.ts`
- `ui/src/lib/run-subscription.ts`
- `ui/src/components/chat/ChatView.tsx`
- `ui/src/components/chat/ChatInput.tsx`
- `ui/src/i18n/en.ts`
- `ui/src/i18n/zh.ts`
- `internal/rpc/control.go`

Proposed additions:
- `ui/src/components/chat/WorkControlBar.tsx`
- `ui/src/components/chat/PlanReview.tsx`
- `ui/src/lib/work-control.test.ts`
- `internal/rpc/work_subscription.go`
- `internal/rpc/work_subscription_test.go`

Do not edit every listed file by default: each diff must implement a task below. All paths are repository-relative; proposed additions are not existing evidence.

## Review Focus

- Crash after durable commit before publication: retry returns committed result, never duplicates work.
- Stale session/ref/submission: conflict cannot be silently retried against a changed objective.
- Approval or question suspension: no new automatic run.
- Permission/profile preservation: collaboration transitions never widen authority.
- Cancellation/delete/shutdown: no resurrected session or hidden continued execution.

The owning scenario tests below and PG-6 cover these risks. Architecture evidence travels with the worker; do not rely on conversation history.

## Task 1: Session replay and activation

Capture a session watermark, replay through it, then stream later committed events with no gap. Client deduplicates session+seq; reconnect refreshes process epoch and activation. Do not persist armed in localStorage.

- [x] Write the scenario test first using existing fixtures in the listed packages. The following is **behavioral pseudocode**, not a claim of executable fixture APIs:
```text
subscription_tests:
  new round commits between replay and live attach -> observed once
  duplicate event -> no duplicate run open
  reconnect after driver restart -> disarmed replaces old armed
  session switch before response -> old response ignored
```
- [x] Run the focused check and observe the intended missing-behavior failure; distinguish missing environment from a valid failing test.
- [x] Implement the smallest change following the shared signatures and ordering in PG-D1/accepted PG-D2.
- [x] Re-run the scenario checks; inspect persisted records and externally visible state, not just reducer return values.
- [x] Review diff for scope and shared-contract consistency. Record evidence; create a human-attributed commit only when version-control work is authorized.

## Task 2: GUI actions

Implement controls in chat; do not repurpose the demo planning page. Keep TodoProgressStrip and task panel intact. Show stopping_for_plan until confirmed. Render ordinary execute vs start Goal choices distinctly and display independent permission state. Use EN default and ZH translations.

- [ ] Write the scenario test first using existing fixtures in the listed packages. The following is **behavioral pseudocode**, not a claim of executable fixture APIs:
```text
ui_tests:
  pending plan opens exact submission; stale action shows conflict
  active/disarmed Goal offers Resume
  armed Goal offers Pause; Enter Plan shows stopping transition
  round-limit displays reason and cannot silently reset count
  next automatic run appears after prior per-run subscription ended
  Todo all completed does not complete Goal
```
- [ ] Run the focused check and observe the intended missing-behavior failure; distinguish missing environment from a valid failing test.
- [ ] Implement the smallest change following the shared signatures and ordering in PG-D1/accepted PG-D2.
- [x] Re-run the scenario checks; inspect persisted records and externally visible state, not just reducer return values.
- [x] Review diff for scope and shared-contract consistency. Record evidence; create a human-attributed commit only when version-control work is authorized.

## Verification

```text
cd ui && pnpm typecheck
cd ui && pnpm test
cd ui && pnpm build
just ci
```

Expected: Run browser acceptance through just dev at port 3015; UI pre-hooks require Go for generated staging. Missing Go is a blocker, not permission to bypass staging.

For required product-contract changes, package tests supplement rather than replace `just ci`. A PostgreSQL skip and a missing toolchain must be reported explicitly.

## Acceptance and handoff

Return the tested revision, changed file list, each scenario result, logs, deviations and remaining blockers. Supervisor checks R8, R2, R5 against PG-D1 and integration evidence before updating the index.

Scope boundary: No generated UI edits, extra transport/store, demo-api imports, Studio overlay work or new Todo storage.

Rollback: retain durable evidence and diagnose before retrying a failed mutation. Database rollback requires a pre-upgrade backup; never erase work events to force an older binary to run.
