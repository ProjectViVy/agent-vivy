# Task 5 G0 result: NO-GO

## Outcome

G0 is **NO-GO** at the correction baseline
`9f26f091f851bfdbe3b878d2cf8480320192ea53`. No production runtime code was
added and no downstream Issue #39 task is unblocked.

The pinned source inspection establishes a design concern, not a reproduced
crash: `internal/runtime/toolbroker.go` directly invokes `tool.InvokableRun`,
and the broker API has no durable operation/result identity or store argument.
The existing runtime contract records potentially executed tool failures as
`effects=unknown` in `TestToolFailureUnknownEffectsCountOnce`.

`TestGraphConformanceBrokerReplayRisk` is an intentionally failing bounded
probe. It calls the real `ExecuteBrokerTool` API twice with the same run and
input in one process, then observes that its in-memory fixture counter is `2`.
It has no Service, Engine, checkpoint, fresh store, injected crash/restart, or
real external side effect. Therefore it does not demonstrate a replayed stable
operation and does not independently prove an external-effect crash window.
Its defensible observation is narrower: repeated direct calls through this API
re-invoke the fixture, and the API seam supplies no durable operation/result
identity for returning a prior result.

G0 requires the missing Service/Eino/checkpoint/fresh-store crash/recovery
criteria, including a real crash/restart effect-safety test. Those criteria and
replay safety remain unproven, so the result is conservatively terminal NO-GO.
Adding a durable idempotency/result boundary or a cross-backend conformance
scenario is later durability/schema work and remains outside Task 5.

## Eino capability check

Inspected the repository-pinned local source only:

- `compose/workflow.go`: Workflow uses the native all-predecessor DAG and
  exposes `End`, `AddInput`, and `AddDependency`.
- `compose/checkpoint.go`: `WithCheckPointStore` and `WithCheckPointID` store
  opaque serialized graph state through `CheckPointStore.Set`.
- `compose/graph_run.go`: `handleInterrupt`/`handleInterruptWithSubGraphAndRerunNodes`
  resolve completed work, construct the checkpoint, and then call
  `checkPointer.set`. The API does not transactionally couple an external tool
  effect to Vivy's Journal or blob store.
- `compose/interrupt.go` and `compose/resume.go`: stateful interrupt and
  targeted resume are supported, but they do not add effect idempotency.

These APIs describe scheduling and checkpoint capabilities. Task 5 did not
exercise them through a Service-owned crash/recovery flow, so it makes no
claim that they establish external-effect exactly-once behavior.

## Plan-vs-need decision

The plan allowed either a minimum native adapter or an evidence-backed NO-GO.
The six G0 criteria are conjunctive. This correction does not infer failure of
the unrun crash test from the fixture counter; it records those criteria as
unproven and keeps the safety gate closed.

Alternatives rejected:

- A Service-owned fresh-store crash/restart test: required for G0, but not
  implemented in Task 5.
- An idempotent test-only fixture: it would prove fixture behavior, not
  arbitrary governed brokered effects.
- An effect table or transactional outbox: this broadens Task 5 into later
  schema and cross-backend work forbidden by the gate instructions.

## Scope and non-goals

No scheduler, child loop, graph lifecycle, schema, migration, RPC, UI, or
fallback runtime was added. The existing intentional RED test is now labeled
accurately as a same-process in-memory broker probe. Tasks 6–14 were not run.
