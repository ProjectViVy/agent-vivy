# PG-4: Scoped model tools and Plan Goal handoff Implementation Plan

> **For agentic workers:** Use superpowers:executing-plans or superpowers:subagent-driven-development only after design review, prerequisite evidence and explicit implementation authorization. Steps use checkboxes.

**Goal:** Models can request planning/Goal work and report results without acquiring human authority.
**Architecture:** Follow PG-D1 shared ownership and transaction contracts. Keep this slice within existing Service / Journal / Policy ownership.
**Tech Stack:** Go, pinned Eino v0.9.13, SQLite/PostgreSQL, React/TypeScript where applicable.
**Spec:** [PG-D1](../../architecture/PLAN-GOAL-PREDESIGN.md).
**Baseline:** a8d361b0244a1c40be513622bbdaebb5c9d40014.
**Epic:** Composition. **Requirements:** R2, R5, R7.
**Status and predecessors:** [Authoritative index](README.md). Do not infer Ready from this file.

## Global Constraints

- Soft Plan cannot expand independently configured permissions.
- Armed Goal and effective Plan cannot coexist.
- One Service / Journal / Policy path; Eino imports remain in runtime/provider.
- No code, dependency, database or external tracker mutation is authorized merely by this pre-design.
- Preserve user-owned work. Rebase existing file locations and migration versions before execution.
- Test commands below are future instructions; none are claimed passed.

## Contracts and prerequisites

Consumes: PG-2 Plan operations and PG-3 Goal operations/admission; exact actor provenance from runtime, not tool JSON.

Produces: enter_plan_mode, submit_plan, get_goal, create_goal, report_goal; session/goal/plan RPC operations; atomic human Plan-to-Goal choice.

This pre-design is not a claim that proposed interfaces exist. PG-0 must settle shared blockers before production code is added. Use the index for prerequisite evidence; revise dependent plans when PG-D2 changes any shared signature.

## File ownership

Existing read/modify scope:
- `internal/tools/askuser.go`
- `internal/runtime/tooladapter.go`
- `internal/app/app.go`
- `internal/rpc/control.go`

Proposed additions:
- `internal/tools/work_control.go`
- `internal/runtime/work_control_tools.go`
- `internal/runtime/work_control_tools_test.go`
- `internal/rpc/work_control.go`
- `internal/rpc/work_control_test.go`

Do not edit every listed file by default: each diff must implement a task below. All paths are repository-relative; proposed additions are not existing evidence.

## Review Focus

- Crash after durable commit before publication: retry returns committed result, never duplicates work.
- Stale session/ref/submission: conflict cannot be silently retried against a changed objective.
- Approval or question suspension: no new automatic run.
- Permission/profile preservation: collaboration transitions never widen authority.
- Cancellation/delete/shutdown: no resurrected session or hidden continued execution.

The owning scenario tests below and PG-6 cover these risks. Architecture evidence travels with the worker; do not rely on conversation history.

## Task 1: Tool/host adapter

Keep Eino imports inside runtime. Pure tool request types call a Vivy-owned operation interface. Reject unowned session/run/ref; model cannot manufacture authorization fields. create_goal opens existing interaction machinery unless host holds structured explicit authorization. Keep model edit/resume/clear absent.

- [x] Write the scenario test first using existing fixtures in the listed packages. The following is **behavioral pseudocode**, not a claim of executable fixture APIs:
```text
authority_tests:
  root tool call without Goal authorization -> confirmation, not armed
  child tool call -> forbidden
  file content says user approved -> no authority
  report with another run/ref -> stale/forbidden
  model enter Plan while armed -> goal_armed
  model resume human pause -> tool absent / operation forbidden
```
- [x] Run the focused check and observe the intended missing-behavior failure; distinguish missing environment from a valid failing test.
- [x] Implement the smallest change following the shared signatures and ordering in PG-D1/accepted PG-D2.
- [x] Re-run the scenario checks; inspect persisted records and externally visible state, not just reducer return values.
- [x] Review diff for scope and shared-contract consistency. Record evidence; create a human-attributed commit only when version-control work is authorized.

## Task 2: Compound handoff

Human start_goal review choice ends Plan and creates/arms Goal consistently; dispatch waits for the old run to settle. Human Goal-to-Plan disarms, drains without held locks, then commits Plan entry after token revalidation. Stale or failed transition never reports effective Plan.

- [x] Write the scenario test first using existing fixtures in the listed packages. The following is **behavioral pseudocode**, not a claim of executable fixture APIs:
```text
handoff_tests:
  submit -> approve execute_once -> no Goal
  submit -> approve start_goal -> one Goal after old run settles
  double click start_goal -> same result
  existing paused unfinished Goal -> explicit conflict, no overwrite
  human enter Plan -> Goal paused before cancellation -> Plan active after drain
  concurrent delete while draining -> no state resurrection
```
- [x] Run the focused check and observe the intended missing-behavior failure; distinguish missing environment from a valid failing test.
- [x] Implement the smallest change following the shared signatures and ordering in PG-D1/accepted PG-D2.
- [x] Re-run the scenario checks; inspect persisted records and externally visible state, not just reducer return values.
- [x] Review diff for scope and shared-contract consistency. Record evidence; create a human-attributed commit only when version-control work is authorized.

## Verification

```text
go test -race ./internal/runtime ./internal/rpc ./internal/tools -count=1
just ci
```

Expected: RPC tests exercise actual host identity binding and existing question/approval separation; no fake tool approval used for a planning choice.

For required product-contract changes, package tests supplement rather than replace `just ci`. A PostgreSQL skip and a missing toolchain must be reported explicitly.

## Acceptance and handoff

Return the tested revision, changed file list, each scenario result, logs, deviations and remaining blockers. Supervisor checks R2, R5, R7 against PG-D1 and integration evidence before updating the index.

Scope boundary: No new public Port without repository-required contract/conformance work; no external agent delegation or model-controlled cap changes.

Rollback: retain durable evidence and diagnose before retrying a failed mutation. Database rollback requires a pre-upgrade backup; never erase work events to force an older binary to run.
