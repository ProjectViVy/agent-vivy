# PG-6: Cross-path acceptance and release evidence Implementation Plan

> **For agentic workers:** Use superpowers:executing-plans or superpowers:subagent-driven-development only after design review, prerequisite evidence and explicit implementation authorization. Steps use checkboxes.

**Goal:** Demonstrate the complete product contract including restart, migration, governance and live coding.
**Architecture:** Follow PG-D1 shared ownership and transaction contracts. Keep this slice within existing Service / Journal / Policy ownership.
**Tech Stack:** Go, pinned Eino v0.9.13, SQLite/PostgreSQL, React/TypeScript where applicable.
**Spec:** [PG-D1](../../architecture/PLAN-GOAL-PREDESIGN.md).
**Baseline:** a8d361b0244a1c40be513622bbdaebb5c9d40014.
**Epic:** Acceptance. **Requirements:** R1, R2, R3, R4, R5, R6, R7, R8, R9.
**Status and predecessors:** [Authoritative index](README.md). Do not infer Ready from this file.

## Global Constraints

- Soft Plan cannot expand independently configured permissions.
- Armed Goal and effective Plan cannot coexist.
- One Service / Journal / Policy path; Eino imports remain in runtime/provider.
- No code, dependency, database or external tracker mutation is authorized merely by this pre-design.
- Preserve user-owned work. Rebase existing file locations and migration versions before execution.
- Test commands below are future instructions; none are claimed passed.

## Contracts and prerequisites

Consumes: All integrated preceding outputs via PG-5; configured Go/Node/PostgreSQL/browser and explicit live-model environment.

Produces: Reviewed evidence per R1–R9, exact tested revision, remaining limitations and rollout/rollback instructions.

This pre-design is not a claim that proposed interfaces exist. PG-0 must settle shared blockers before production code is added. Use the index for prerequisite evidence; revise dependent plans when PG-D2 changes any shared signature.

## File ownership

Existing read/modify scope:
- `docs/plans/plan-goal/README.md`
- `docs/logs/2026-09-21-plan-goal-predesign/verification.md`

Proposed additions:
- `internal/app/plan_goal_e2e_test.go`

Do not edit every listed file by default: each diff must implement a task below. All paths are repository-relative; proposed additions are not existing evidence.

## Review Focus

- Crash after durable commit before publication: retry returns committed result, never duplicates work.
- Stale session/ref/submission: conflict cannot be silently retried against a changed objective.
- Approval or question suspension: no new automatic run.
- Permission/profile preservation: collaboration transitions never widen authority.
- Cancellation/delete/shutdown: no resurrected session or hidden continued execution.

The owning scenario tests below and PG-6 cover these risks. Architecture evidence travels with the worker; do not rely on conversation history.

## Task 1: Deterministic integration

Boot real application composition with scripted model and disposable database/workspace. Exercise RPC -> model tool -> review -> Goal -> two ordinary runs -> evidence -> completion. Include migration fixtures and restart at transaction/cancellation boundaries.

- [ ] Write the scenario test first using existing fixtures in the listed packages. The following is **behavioral pseudocode**, not a claim of executable fixture APIs:
```text
scenario:
  Plan saves design under writable policy
  user approves Goal with finite cap
  round 1 changes code; round 2 validates result
  report complete links actual test/build run event
  replay after restart -> same result, disarmed
  repeat under read-only -> write denied; no approval bypass
  legacy Plan resume -> old hard restriction preserved
```
- [ ] Run the focused check and observe the intended missing-behavior failure; distinguish missing environment from a valid failing test.
- [ ] Implement the smallest change following the shared signatures and ordering in PG-D1/accepted PG-D2.
- [ ] Re-run the scenario checks; inspect persisted records and externally visible state, not just reducer return values.
- [ ] Review diff for scope and shared-contract consistency. Record evidence; create a human-attributed commit only when version-control work is authorized.

## Task 2: Product and live evidence

Run the repository gate and configured Postgres gate. Start split dev UI for browser walkthrough. Run one bounded real coding objective only in a disposable project; report exact command exits and changed files. A model saying success is not evidence. Include manual pause, failed provider and re-planning controls.

- [ ] Write the scenario test first using existing fixtures in the listed packages. The following is **behavioral pseudocode**, not a claim of executable fixture APIs:
```text
evidence_record:
  revision: actual tested commit
  checks: command, exit status, log location
  database: SQLite and configured PostgreSQL
  browser: user actions and observed states
  live: objective, cap, run IDs, file changes, validation results
  limitations: every skipped or failing check
```
- [ ] Run the focused check and observe the intended missing-behavior failure; distinguish missing environment from a valid failing test.
- [ ] Implement the smallest change following the shared signatures and ordering in PG-D1/accepted PG-D2.
- [ ] Re-run the scenario checks; inspect persisted records and externally visible state, not just reducer return values.
- [ ] Review diff for scope and shared-contract consistency. Record evidence; create a human-attributed commit only when version-control work is authorized.

## Verification

```text
just ci
just test-postgres
just dev
```

Expected: just dev is a long-running development entry, not a passing test. Record browser assertions separately. Only accepted integrated evidence changes index status to Done.

For required product-contract changes, package tests supplement rather than replace `just ci`. A PostgreSQL skip and a missing toolchain must be reported explicitly.

## Acceptance and handoff

Return the tested revision, changed file list, each scenario result, logs, deviations and remaining blockers. Supervisor checks R1, R2, R3, R4, R5, R6, R7, R8, R9 against PG-D1 and integration evidence before updating the index.

Scope boundary: No merge/deploy/push authorized by this plan. Do not modify acceptance to conceal unavailable environment or live effectiveness failures.

Rollback: retain durable evidence and diagnose before retrying a failed mutation. Database rollback requires a pre-upgrade backup; never erase work events to force an older binary to run.
