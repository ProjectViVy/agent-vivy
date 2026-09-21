# PG-0: Resolve adapter and history contracts Implementation Plan

> **For agentic workers:** Use superpowers:executing-plans or superpowers:subagent-driven-development only after design review, prerequisite evidence and explicit implementation authorization. Steps use checkboxes.

**Goal:** Replace the two PG-D1 blockers with source-backed, executable decisions.
**Architecture:** Follow PG-D1 shared ownership and transaction contracts. Keep this slice within existing Service / Journal / Policy ownership.
**Tech Stack:** Go, pinned Eino v0.9.13, SQLite/PostgreSQL, React/TypeScript where applicable.
**Spec:** [PG-D1](../../architecture/PLAN-GOAL-PREDESIGN.md).
**Baseline:** aeec3b59233c45a5bcfd50c1ed2b0ac862d5e7eb.
**Epic:** Foundation. **Requirements:** R1, R2, R6, R7, R9.
**Status and predecessors:** [Authoritative index](README.md). Do not infer Ready from this file.

## Global Constraints

- Soft Plan cannot expand independently configured permissions.
- Armed Goal and effective Plan cannot coexist.
- One Service / Journal / Policy path; Eino imports remain in runtime/provider.
- No code, dependency, database or external tracker mutation is authorized merely by this pre-design.
- Preserve user-owned work. Rebase existing file locations and migration versions before execution.
- Test commands below are future instructions; none are claimed passed.

## Contracts and prerequisites

Consumes: PG-D1 sections 2, 8 and 9; pinned Eino v0.9.13 and repository rules.

Produces: PG-D2: exact Eino prompt/review/stop API; message-to-work-event ordering anchor; history/counter rules; shared bounds and config source. Probe evidence in the existing delivery log.

This pre-design is not a claim that proposed interfaces exist. PG-0 must settle shared blockers before production code is added. Use the index for prerequisite evidence; revise dependent plans when PG-D2 changes any shared signature.

## File ownership

Existing read/modify scope:
- `docs/architecture/PLAN-GOAL-PREDESIGN.md`
- `docs/plans/plan-goal/README.md`
- `docs/eino-capability-verify.md`
- `internal/runtime/engine.go`
- `internal/runtime/tooladapter.go`
- `internal/storage/sqlite/history_mutations.go`
- `internal/storage/postgres/history_mutations.go`

Proposed additions:
- `internal/runtime/plan_goal_probe_test.go`

Do not edit every listed file by default: each diff must implement a task below. All paths are repository-relative; proposed additions are not existing evidence.

## Review Focus

- Crash after durable commit before publication: retry returns committed result, never duplicates work.
- Stale session/ref/submission: conflict cannot be silently retried against a changed objective.
- Approval or question suspension: no new automatic run.
- Permission/profile preservation: collaboration transitions never widen authority.
- Cancellation/delete/shutdown: no resurrected session or hidden continued execution.

The owning scenario tests below and PG-6 cover these risks. Architecture evidence travels with the worker; do not rely on conversation history.

## Task 1: Pinned Eino capability proof

Read the actual v0.9.13 upstream packages adk and components/tool, then exercise the existing scripted-model test harness. Prove entry guidance applies before the next model request, review waits without admitting another run, and resume addresses the exact call. Prove completion/block reporting prevents later effectful calls from the same batch. If an API is absent, record the observed gap; do not write a substitute loop.

- [ ] Write the scenario test first using existing fixtures in the listed packages. The following is **behavioral pseudocode**, not a claim of executable fixture APIs:
```text
probe_script:
  model emits enter_plan_mode
  next model request must contain planning guidance exactly once
  model emits submit_plan(markdown) plus write_file in same batch
  before human decision: zero write_file executions
  decide exact submission -> resumes once
  repeat decision -> no second resume
  report_goal(completed) plus write_file -> no post-report write
```
- [ ] Run the focused check and observe the intended missing-behavior failure; distinguish missing environment from a valid failing test.
- [ ] Implement the smallest change following the shared signatures and ordering in PG-D1/accepted PG-D2.
- [ ] Re-run the scenario checks; inspect persisted records and externally visible state, not just reducer return values.
- [ ] Review diff for scope and shared-contract consistency. Record evidence; create a human-attributed commit only when version-control work is authorized.

## Task 2: History and limits contract

Use the existing edit/fork transaction implementation as the baseline. Choose a durable message/work-sequence anchor that can represent host changes between messages. Add same-timestamp test data. Update PG-D1 with exact storage fields and FK/uniqueness rules. Define a non-refundable per-Goal usage floor distinct from visible prefix state if needed; count it as a derived fact, not a second mutable counter. Resolve payload/config bounds by naming their real configuration owner.

- [ ] Write the scenario test first using existing fixtures in the listed packages. The following is **behavioral pseudocode**, not a claim of executable fixture APIs:
```text
history_probe:
  message A; create Goal; admit rounds 1 and 2; message B
  rewind to A -> no automatic activation, spent work not refunded
  fork through B -> source unchanged, copied context disarmed
  interleave plan decision and message at same timestamp
  replay twice -> same prefix and no inherited approval
  resumption cannot refer to another session's pending submission
```
- [ ] Run the focused check and observe the intended missing-behavior failure; distinguish missing environment from a valid failing test.
- [ ] Implement the smallest change following the shared signatures and ordering in PG-D1/accepted PG-D2.
- [ ] Re-run the scenario checks; inspect persisted records and externally visible state, not just reducer return values.
- [ ] Review diff for scope and shared-contract consistency. Record evidence; create a human-attributed commit only when version-control work is authorized.

## Verification

```text
go test ./internal/runtime -run 'TestPlanGoalProbe' -count=1
just test-postgres
just ci
```

Expected: Probe tests must run with a real Go toolchain. An unset VIVY_POSTGRES_TEST_DSN is not evidence. Only after accepted source and runtime evidence may the supervisor replace blockers and release PG-1/PG-2.

For required product-contract changes, package tests supplement rather than replace `just ci`. A PostgreSQL skip and a missing toolchain must be reported explicitly.

## Acceptance and handoff

Return the tested revision, changed file list, each scenario result, logs, deviations and remaining blockers. Supervisor checks R1, R2, R6, R7, R9 against PG-D1 and integration evidence before updating the index.

Scope boundary: Do not implement Goal products during the probe. A discovered need for a new engine, public Port or weaker history/authority contract requires escalation.

Rollback: retain durable evidence and diagnose before retrying a failed mutation. Database rollback requires a pre-upgrade backup; never erase work events to force an older binary to run.
