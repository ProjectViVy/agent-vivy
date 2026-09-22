# PG-3: Goal lifecycle and unified admission Implementation Plan

> **For agentic workers:** Use superpowers:executing-plans or superpowers:subagent-driven-development only after design review, prerequisite evidence and explicit implementation authorization. Steps use checkboxes.

**Goal:** At most one authorized continuation is admitted while human requests, cancellation and existing budgets remain authoritative.
**Architecture:** Follow PG-D1 shared ownership and transaction contracts. Keep this slice within existing Service / Journal / Policy ownership.
**Tech Stack:** Go, pinned Eino v0.9.13, SQLite/PostgreSQL, React/TypeScript where applicable.
**Spec:** [PG-D1](../../architecture/PLAN-GOAL-PREDESIGN.md).
**Baseline:** a8d361b0244a1c40be513622bbdaebb5c9d40014.
**Epic:** Goal continuation. **Requirements:** R3, R4, R5, R6, R9.
**Status and predecessors:** [Authoritative index](README.md). Do not infer Ready from this file.

## Global Constraints

- Soft Plan cannot expand independently configured permissions.
- Armed Goal and effective Plan cannot coexist.
- One Service / Journal / Policy path; Eino imports remain in runtime/provider.
- No code, dependency, database or external tracker mutation is authorized merely by this pre-design.
- Preserve user-owned work. Rebase existing file locations and migration versions before execution.
- Test commands below are future instructions; none are claimed passed.

## Contracts and prerequisites

Consumes: PG-1 CommitGoalRun and reducer; existing Service.runWithOptions persistence callback, recovery and cancellation.

Produces: Goal lifecycle host operations; per-session startup arbitration; coalesced wake-up; process-local activation; WorkView; current-run settled signal.

This pre-design is not a claim that proposed interfaces exist. PG-0 must settle shared blockers before production code is added. Use the index for prerequisite evidence; revise dependent plans when PG-D2 changes any shared signature.

## File ownership

Existing read/modify scope:
- `internal/runtime/service.go`
- `internal/runtime/cron_scheduler.go`
- `internal/runtime/shell.go`
- `internal/app/app.go`
- `internal/rpc/control.go`

Proposed additions:
- `internal/runtime/work_admission.go`
- `internal/runtime/goal_driver.go`
- `internal/runtime/goal_driver_test.go`
- `internal/runtime/work_admission_test.go`

Do not edit every listed file by default: each diff must implement a task below. All paths are repository-relative; proposed additions are not existing evidence.

## Review Focus

- Crash after durable commit before publication: retry returns committed result, never duplicates work.
- Stale session/ref/submission: conflict cannot be silently retried against a changed objective.
- Approval or question suspension: no new automatic run.
- Permission/profile preservation: collaboration transitions never widen authority.
- Cancellation/delete/shutdown: no resurrected session or hidden continued execution.

The owning scenario tests below and PG-6 cover these risks. Architecture evidence travels with the worker; do not rely on conversation history.

## Task 1: Admission boundary

Register all primary producers at one session startup gate. Human queue priority applies to requests registered at that gate. Preserve existing producer-specific success/error contracts; queueing requires a visible ticket or existing response behavior, not a fabricated accepted RunID. Resolve precise RPC ticket schema in the shared contract before shipping. Never hold the gate across model execution or drain.

- [ ] Write the scenario test first using existing fixtures in the listed packages. The following is **behavioral pseudocode**, not a claim of executable fixture APIs:
```text
admit_auto(candidate):
  lock session
  require not deleting/stopping
  require no pending human work or suspended/active primary
  require Plan inactive and activation generation unchanged
  read latest state; validate ref/version/remaining cap
  records = existing Service startup preparation
  result = CommitGoalRun(records)
  unlock session
  existing Service activates committed run exactly once
```
- [ ] Run the focused check and observe the intended missing-behavior failure; distinguish missing environment from a valid failing test.
- [ ] Implement the smallest change following the shared signatures and ordering in PG-D1/accepted PG-D2.
- [ ] Re-run the scenario checks; inspect persisted records and externally visible state, not just reducer return values.
- [ ] Review diff for scope and shared-contract consistency. Record evidence; create a human-attributed commit only when version-control work is authorized.

## Task 2: Lifecycle and cancellation

Create/resume arms only after successful durable mutation. Pause/clear/cancel disarm before cancellation. Old-revision reports fail CAS. Errors stop; normal terminal wakes only after cleanup. Reuse per-run ledgers and attach reports to real admitted run identities.

- [ ] Write the scenario test first using existing fixtures in the listed packages. The following is **behavioral pseudocode**, not a claim of executable fixture APIs:
```text
race_tests:
  barrier before CommitGoalRun; register human -> human wins
  two wake-ups -> one admitted run
  pause at reservation -> no admission
  edit during run -> old report rejected
  approval pending -> no new round
  Cancel current run -> no immediate replacement
  round cap reached -> blocked(round-limit)
  crash after commit -> fail-closed recovery, no refund or replay
```
- [ ] Run the focused check and observe the intended missing-behavior failure; distinguish missing environment from a valid failing test.
- [ ] Implement the smallest change following the shared signatures and ordering in PG-D1/accepted PG-D2.
- [ ] Re-run the scenario checks; inspect persisted records and externally visible state, not just reducer return values.
- [ ] Review diff for scope and shared-contract consistency. Record evidence; create a human-attributed commit only when version-control work is authorized.

## Task 3: Recovery and shutdown

Recover existing runs before considering Goal state. Build live activation as disarmed. During shutdown stop accepting automatic work, cancel/drain then close backend. Surface failure to persist stop; do not claim paused if durability failed.

- [ ] Write the scenario test first using existing fixtures in the listed packages. The following is **behavioral pseudocode**, not a claim of executable fixture APIs:
```text
shutdown_tests:
  wake races shutdown -> no new engine after stop begins
  pause storage error -> disarmed plus explicit error
  restart pending Question -> pending remains, Goal stays disarmed
  nonterminal child -> existing worker-lost handling preserved
```
- [ ] Run the focused check and observe the intended missing-behavior failure; distinguish missing environment from a valid failing test.
- [ ] Implement the smallest change following the shared signatures and ordering in PG-D1/accepted PG-D2.
- [ ] Re-run the scenario checks; inspect persisted records and externally visible state, not just reducer return values.
- [ ] Review diff for scope and shared-contract consistency. Record evidence; create a human-attributed commit only when version-control work is authorized.

## Verification

```text
go test -race ./internal/runtime -run 'Test(Goal|WorkAdmission)' -count=1
just ci
```

Expected: Deterministic barriers verify race ordering without timing sleeps. Ordinary Service and governance tests remain green.

For required product-contract changes, package tests supplement rather than replace `just ci`. A PostgreSQL skip and a missing toolchain must be reported explicitly.

## Acceptance and handoff

Return the tested revision, changed file list, each scenario result, logs, deviations and remaining blockers. Supervisor checks R3, R4, R5, R6, R9 against PG-D1 and integration evidence before updating the index.

Scope boundary: No second loop, advisory-hook reentrancy, Cron polling, aggregate budget claims, automatic retries or future scheduling service.

Rollback: retain durable evidence and diagnose before retrying a failed mutation. Database rollback requires a pre-upgrade backup; never erase work events to force an older binary to run.
