# PG-1: Session work journal and atomic commits Implementation Plan

> **For agentic workers:** Use superpowers:executing-plans or superpowers:subagent-driven-development only after design review, prerequisite evidence and explicit implementation authorization. Steps use checkboxes.

**Goal:** Durably replay work state and atomically account ordinary Goal runs on both backends.
**Architecture:** Follow PG-D1 shared ownership and transaction contracts. Keep this slice within existing Service / Journal / Policy ownership.
**Tech Stack:** Go, pinned Eino v0.9.13, SQLite/PostgreSQL, React/TypeScript where applicable.
**Spec:** [PG-D1](../../architecture/PLAN-GOAL-PREDESIGN.md).
**Baseline:** a8d361b0244a1c40be513622bbdaebb5c9d40014.
**Epic:** Foundation. **Requirements:** R3, R6, R9.
**Status and predecessors:** [Authoritative index](README.md). Do not infer Ready from this file.

## Global Constraints

- Soft Plan cannot expand independently configured permissions.
- Armed Goal and effective Plan cannot coexist.
- One Service / Journal / Policy path; Eino imports remain in runtime/provider.
- No code, dependency, database or external tracker mutation is authorized merely by this pre-design.
- Preserve user-owned work. Rebase existing file locations and migration versions before execution.
- Test commands below are future instructions; none are claimed passed.

## Contracts and prerequisites

Consumes: Accepted PG-0 ordering anchor and PG-D2 types; existing domain.Message, Run, RunEvent.

Produces: ReadWork, ReplayWork, CommitWork, CommitGoalRun and WorkMutation/GoalRunAdmission/WorkCommitResult exactly as the shared design; SQLite/Postgres migration and history integration.

This pre-design is not a claim that proposed interfaces exist. PG-0 must settle shared blockers before production code is added. Use the index for prerequisite evidence; revise dependent plans when PG-D2 changes any shared signature.

## File ownership

Existing read/modify scope:
- `internal/storage/contracts.go`
- `internal/storage/sqlite/sqlite.go`
- `internal/storage/postgres/schema.go`
- `internal/storage/sqlite/history_mutations.go`
- `internal/storage/postgres/history_mutations.go`
- `internal/storage/conformance/suite.go`

Proposed additions:
- `internal/domain/work_control.go`
- `internal/domain/work_control_test.go`
- `internal/storage/work_control.go`
- `internal/storage/sqlite/work_control.go`
- `internal/storage/postgres/work_control.go`
- `internal/storage/conformance/work_control.go`

Do not edit every listed file by default: each diff must implement a task below. All paths are repository-relative; proposed additions are not existing evidence.

## Review Focus

- Crash after durable commit before publication: retry returns committed result, never duplicates work.
- Stale session/ref/submission: conflict cannot be silently retried against a changed objective.
- Approval or question suspension: no new automatic run.
- Permission/profile preservation: collaboration transitions never widen authority.
- Cancellation/delete/shutdown: no resurrected session or hidden continued execution.

The owning scenario tests below and PG-6 cover these risks. Architecture evidence travels with the worker; do not rely on conversation history.

## Task 1: Reducer and schema

Implement pure strict event folding and lifecycle validation first. Use the next free migration number after rebasing, not a hard-coded reservation of migration 024. Reject invalid schema versions, stale expected seq, duplicate request with changed hash and cross-session references. Extend the existing storage engine contract only where the app needs the capability.

- [ ] Write the scenario test first using existing fixtures in the listed packages. The following is **behavioral pseudocode**, not a claim of executable fixture APIs:
```text
fold(events):
  state = empty
  for event in sequence:
    require event.seq == state.version + 1
    require supported payload schema
    validate transition and exact goal reference
    if round_admitted: require round == previous_count + 1 <= cap
    apply event
  return state
```
- [ ] Run the focused check and observe the intended missing-behavior failure; distinguish missing environment from a valid failing test.
- [ ] Implement the smallest change following the shared signatures and ordering in PG-D1/accepted PG-D2.
- [ ] Re-run the scenario checks; inspect persisted records and externally visible state, not just reducer return values.
- [ ] Review diff for scope and shared-contract consistency. Record evidence; create a human-attributed commit only when version-control work is authorized.

## Task 2: Atomic round and review writes

Implement the transaction in design section 5. Return original result for an identical committed retry even if its expectation is now old. Request IDs must be recorded for no-op decisions too. Validate review ownership and settle its Question linkage inside the same transaction. Use session lock semantics appropriate to each backend; no recursive backend method call through the one-connection SQLite pool.

- [ ] Write the scenario test first using existing fixtures in the listed packages. The following is **behavioral pseudocode**, not a claim of executable fixture APIs:
```text
transaction_tests:
  inject failure after message insertion -> no message/run/round remains
  inject failure after run.started -> no partial startup remains
  commit success then retry same ID -> same run ID, one round
  same ID with different objective -> conflict
  two distinct IDs at same expected version -> one winner
  cancelled review cannot arm Goal
  malformed replay record -> explicit fault, no repaired state
```
- [ ] Run the focused check and observe the intended missing-behavior failure; distinguish missing environment from a valid failing test.
- [ ] Implement the smallest change following the shared signatures and ordering in PG-D1/accepted PG-D2.
- [ ] Re-run the scenario checks; inspect persisted records and externally visible state, not just reducer return values.
- [ ] Review diff for scope and shared-contract consistency. Record evidence; create a human-attributed commit only when version-control work is authorized.

## Task 3: History and delete

Implement the accepted PG-0 anchor rules in existing history transactions. Extend deletion to new owned rows. Original history remains append-only; no synthetic permanently active run. Ensure migration of empty and existing databases leaves prior run events untouched.

- [ ] Write the scenario test first using existing fixtures in the listed packages. The following is **behavioral pseudocode**, not a claim of executable fixture APIs:
```text
history_checks:
  new schema opens old DB without rewriting run.started
  fork source rows unchanged
  rewind retains usage floor from accepted contract
  session delete leaves no orphan work or submission rows
```
- [ ] Run the focused check and observe the intended missing-behavior failure; distinguish missing environment from a valid failing test.
- [ ] Implement the smallest change following the shared signatures and ordering in PG-D1/accepted PG-D2.
- [ ] Re-run the scenario checks; inspect persisted records and externally visible state, not just reducer return values.
- [ ] Review diff for scope and shared-contract consistency. Record evidence; create a human-attributed commit only when version-control work is authorized.

## Verification

```text
go test ./internal/domain ./internal/storage/sqlite ./internal/storage/conformance -count=1
just test-postgres
just ci
```

Expected: Equivalent conformance behavior on SQLite and configured PostgreSQL; transaction injection demonstrates rollback, not just successful reducer tests.

For required product-contract changes, package tests supplement rather than replace `just ci`. A PostgreSQL skip and a missing toolchain must be reported explicitly.

## Acceptance and handoff

Return the tested revision, changed file list, each scenario result, logs, deviations and remaining blockers. Supervisor checks R3, R6, R9 against PG-D1 and integration evidence before updating the index.

Scope boundary: No runtime driver, UI, general event bus or duplicate Goal database. No automatic downgrade by deleting new tables.

Rollback: retain durable evidence and diagnose before retrying a failed mutation. Database rollback requires a pre-upgrade backup; never erase work events to force an older binary to run.
