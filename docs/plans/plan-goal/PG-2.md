# PG-2: Soft Plan and exact submission review Implementation Plan

> **For agentic workers:** Use superpowers:executing-plans or superpowers:subagent-driven-development only after design review, prerequisite evidence and explicit implementation authorization. Steps use checkboxes.

**Goal:** Plan uses guidance with independent permissions and exact human-reviewed transitions.
**Architecture:** Follow PG-D1 shared ownership and transaction contracts. Keep this slice within existing Service / Journal / Policy ownership.
**Tech Stack:** Go, pinned Eino v0.9.13, SQLite/PostgreSQL, React/TypeScript where applicable.
**Spec:** [PG-D1](../../architecture/PLAN-GOAL-PREDESIGN.md).
**Baseline:** aeec3b59233c45a5bcfd50c1ed2b0ac862d5e7eb.
**Epic:** Plan collaboration. **Requirements:** R1, R2, R5, R7, R9.
**Status and predecessors:** [Authoritative index](README.md). Do not infer Ready from this file.

## Global Constraints

- Soft Plan cannot expand independently configured permissions.
- Armed Goal and effective Plan cannot coexist.
- One Service / Journal / Policy path; Eino imports remain in runtime/provider.
- No code, dependency, database or external tracker mutation is authorized merely by this pre-design.
- Preserve user-owned work. Rebase existing file locations and migration versions before execution.
- Test commands below are future instructions; none are claimed passed.

## Contracts and prerequisites

Consumes: PG-1 work journal; PG-0 proved Eino boundary; existing policy snapshots and Question lifecycle.

Produces: Host operations EnterPlan, LeavePlan, SubmitPlan, DecidePlan bound to WorkExpectation and PlanDecision; versioned collaboration metadata with explicit legacy interpretation.

This pre-design is not a claim that proposed interfaces exist. PG-0 must settle shared blockers before production code is added. Use the index for prerequisite evidence; revise dependent plans when PG-D2 changes any shared signature.

## File ownership

Existing read/modify scope:
- `internal/runtime/runmode.go`
- `internal/runtime/policy_context.go`
- `internal/runtime/tooladapter.go`
- `internal/runtime/shell.go`
- `internal/runtime/prompt.go`
- `internal/runtime/engine.go`
- `internal/runtime/payloads.go`
- `internal/domain/question.go`
- `internal/domain/review.go`
- `schemas/events/payloads/run.started.json`

Proposed additions:
- `internal/runtime/work_control.go`
- `internal/runtime/plan_review.go`
- `internal/runtime/prompts/plan.md`
- `internal/runtime/plan_review_test.go`

Do not edit every listed file by default: each diff must implement a task below. All paths are repository-relative; proposed additions are not existing evidence.

## Review Focus

- Crash after durable commit before publication: retry returns committed result, never duplicates work.
- Stale session/ref/submission: conflict cannot be silently retried against a changed objective.
- Approval or question suspension: no new automatic run.
- Permission/profile preservation: collaboration transitions never widen authority.
- Cancellation/delete/shutdown: no resurrected session or hidden continued execution.

The owning scenario tests below and PG-6 cover these risks. Architecture evidence travels with the worker; do not rely on conversation history.

## Task 1: Soft mode and legacy split

Separate collaboration selection from execution policy. New Plan does not coerce a plan policy, but preserve policy explicitly selected independently. Missing collaboration contract version on historical Plan run means legacy hard restriction on every resume path, including direct shell. Embed one Markdown guidance source through existing runtime composition.

- [ ] Write the scenario test first using existing fixtures in the listed packages. The following is **behavioral pseudocode**, not a claim of executable fixture APIs:
```text
effective_restriction(run):
  if legacy(run) and run.mode == plan: enforce old hard Plan behavior
  else: enforce independently pinned policy/grants/sandbox
  if effective collaboration == plan: append plan guidance
  never infer a wider policy from leaving Plan
```
- [ ] Run the focused check and observe the intended missing-behavior failure; distinguish missing environment from a valid failing test.
- [ ] Implement the smallest change following the shared signatures and ordering in PG-D1/accepted PG-D2.
- [ ] Re-run the scenario checks; inspect persisted records and externally visible state, not just reducer return values.
- [ ] Review diff for scope and shared-contract consistency. Record evidence; create a human-attributed commit only when version-control work is authorized.

## Task 2: Exact plan review

Persist immutable full submission and review linkage. Decisions reference submission ID and expected work version. Rejection and expiry leave Plan active. execute_once ends Plan but does not arm Goal; start_goal requires explicit objective and cap and is rejected while an unfinished Goal exists. Keep actual Goal arming adapter disabled until PG-3/PG-4 are integrated; return capability absence rather than simulated success.

- [ ] Write the scenario test first using existing fixtures in the listed packages. The following is **behavioral pseudocode**, not a claim of executable fixture APIs:
```text
decision_tests:
  submit v1; submit v2; decide v1 -> review_stale
  approve exact v2 twice -> same decision, one execution authorization
  cancel/expire review -> no execute authorization
  read-only permission + Plan document write -> denied
  writable policy + planning document save -> allowed
  historical hard-Plan checkpoint + effectful tool -> denied
```
- [ ] Run the focused check and observe the intended missing-behavior failure; distinguish missing environment from a valid failing test.
- [ ] Implement the smallest change following the shared signatures and ordering in PG-D1/accepted PG-D2.
- [ ] Re-run the scenario checks; inspect persisted records and externally visible state, not just reducer return values.
- [ ] Review diff for scope and shared-contract consistency. Record evidence; create a human-attributed commit only when version-control work is authorized.

## Verification

```text
go test ./internal/runtime -run 'Test(Plan|LegacyPlan)' -count=1
just ci
```

Expected: Model/scripted evidence proves guidance timing and review ordering; snapshot tests alone cannot prove no pre-approval execution.

For required product-contract changes, package tests supplement rather than replace `just ci`. A PostgreSQL skip and a missing toolchain must be reported explicitly.

## Acceptance and handoff

Return the tested revision, changed file list, each scenario result, logs, deviations and remaining blockers. Supervisor checks R1, R2, R5, R7, R9 against PG-D1 and integration evidence before updating the index.

Scope boundary: No model self-approval, permission upgrades, standalone review database, Persona/Memory changes or widening through a falsely Readonly tool.

Rollback: retain durable evidence and diagnose before retrying a failed mutation. Database rollback requires a pre-upgrade backup; never erase work events to force an older binary to run.
