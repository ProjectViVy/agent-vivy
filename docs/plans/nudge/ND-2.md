# ND-2: Run Observation and Durable Outcomes Implementation Plan

> **For agentic workers:** Use `superpowers:executing-plans` to implement task-by-task. Use subagent-driven-development only when delegation is separately selected. This plan is not implementation authorization.

**Spec:** [NUDGE-DESIGN.md](../../architecture/NUDGE-DESIGN.md), revision ND-D1.
**Baseline:** a8d361b0244a1c40be513622bbdaebb5c9d40014.
**Status and predecessors:** [index](README.md), the sole status owner.
**Tech stack:** Go 1.26.4, Eino v0.9.13, existing Journal and tool adapters.

## Global constraints

Preserve Service.Run/Journal/policy, Eino import quarantine, native interrupts and existing budgets. No automatic tool replay, new public Port, database migration or additional model request solely for nudge. No production Journal access. Read the index's five review risks; the cases owned here are specified below. Future test code blocks are behavioral pseudocode, not compiled/passing tests.

**Goal:** One bounded run-local detector observes typed outcomes and never loses the executed sixth call from the Journal.
**Architecture:** Move ownership of the existing loopWindow into nudgeState; mapper supplies correlated results and Service seals only after durable batch completion. The model boundary consumes a notice later in ND-3.
**Scope:** State, lifecycle, Journal payloads and schema compatibility. No model prompt injection.

## Files and interfaces

Create `internal/runtime/tool_failure.go` (toolFailure type only; classifier follows in ND-1), `internal/runtime/nudge_state.go`, `internal/runtime/nudge_state_test.go`, `schemas/events/payloads/tool.nudge.json` (proposed). Modify `internal/runtime/loopguard.go`, `mapper.go`, `service.go`, `payloads.go`, `internal/domain/event.go`, `internal/domain/domain_test.go`, `schemas/events/run-event.schema.json`, `schemas/events/payloads/tool.finished.json`, `internal/runtime/loopdetect_test.go`. If schema/event tests enumerate payload files, update their existing fixture lists. Verify `internal/runtime/trajectory.go` and `ui/src/lib/run-rows.ts` keep error projection.

Consumes ND-0 ordering evidence; defines design §4's toolFailure record and uses synthetic typed fixtures. ND-1 later supplies real failure classifiers. Produces every nudgeState method in §4, EventToolNudge, optional tool.finished metadata and the five-field tool.nudge payload in §7. The model wrapper must not access mapper internals directly.

## Task 1 — Single detector and batch lifecycle

- [ ] Write `TestNudgeState` cases before implementation:

```text
six identical unsuccessful singleton batches -> notices 3,5; error at 6
six identical successful calls -> no notice; hard stop still 6
JSON object key-order changes -> same canonical identity
changed result/argument -> changed signature
c2 completes before c1 -> Seal evaluates request order
duplicate result ID, duplicate registration, overlapping batch -> invariant error
Take waits until Seal; cancellation and Abort release it
Take twice without a new batch -> no second notice
new state for resume -> empty window and no pending notice
```

- [ ] Run `go test -timeout 20m ./internal/runtime -run '^TestNudgeState' -count=1`, then implement the design's fixed-size window and admitted-batch map. Extend record to return count/error; remove mapper's old independent detector.
- [ ] Select one notice per batch: highest threshold, ties earliest request index. Never count reminder text or diagnostic formatting changes introduced only by nudge.
- [ ] Copy bounded outcome values; no raw arguments retained beyond the outstanding batch, no Journal I/O while locked. Invariant errors preserve cause and abort the Run.

## Task 2 — Durability and terminal ownership

- [ ] Mapper adds metadata to tool.finished by call ID; keep error non-empty for a failed invocation even when Eino received nil error.
- [ ] Service persists all completed tool outcomes before Seal. Move hard-stop decision to the settled batch boundary; sixth tool.finished remains visible before exactly one run.failed. Update the existing five-finished-event assertion to six with an explicit audit regression test.

```text
consume tool event -> construct event with original result and failure
persist event -> on success record completion
if batch fully persisted: Seal(nil)
if persistence/mapping/budget fails: Abort(cause), cancel drive, emit existing terminal when possible
```

- [ ] Add `TestNudgeJournalFailure` at the last result of a parallel batch. Expected: no waiting model handoff, no stale notice, no deadlock; stored events reflect only successful appends.
- [ ] Ensure drive and every resume path attach a fresh state; Service terminal paths Abort and cancel producers even if consume returns early. Do not change native interrupt/checkpoint ownership.
- [ ] Add strict event schema and optional outcome/reason/effects fields; explicitly include existing parts where multimodal outcome fixtures require it. Preserve old payloads and exact required fields for new payloads. Update domain vocabulary tests only for this addition, not unrelated baseline drift.
- [ ] Run `go test -timeout 20m ./internal/runtime ./internal/domain -run 'TestNudge|TestServiceToolLoop|Test.*Event|Test.*Schema' -count=1` and race-test state/batch code on a supported host.
- [ ] Commit explicit paths and evidence with `feat: journal bounded nudge observations`.

## Acceptance and handoff

Return one-detector ownership proof, event JSON fixtures validated through repository schema tests, six-result-stop trace and cancellation/parallel race output. Readers must accept new vocabulary before ND-3 emits it. Changing existing loop budgets or adding persistent resume counters is outside scope.
