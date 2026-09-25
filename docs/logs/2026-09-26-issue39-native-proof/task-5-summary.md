# Task 5 G0 result: NO-GO

## Outcome

G0 is **NO-GO** at parent commit
`b46998a67b9d17d9ab7cc1c9f27ad4c22506fe23`. No production runtime code was
added and no downstream Issue #39 task is unblocked.

The decisive missing contract is crash-safe effect identity. In pinned Eino
v0.9.13, `compose/graph_run.go:502-568` builds and persists Workflow state in
`handleInterrupt`; the actual `checkPointer.set` occurs at line 561 after the
completed batch has been resolved. Vivy's child path records `tool.started` at
`internal/app/worker.go:577`, calls `ExecuteBrokerTool` at line 580, and only
then attempts `tool.finished` at line 585. That completion persistence error
is discarded. `internal/runtime/toolbroker.go:26-91` validates authority and
then directly calls `tool.InvokableRun` at line 83; its signature carries no
tool-call/operation ID or durable store.
Consequently a process loss after the external effect but before a later Eino
checkpoint or Journal completion is observationally identical to a loss before
the effect. Retrying can duplicate the effect; refusing to retry can lose it.
The existing runtime contract accurately calls comparable tool uncertainty
`effects=unknown` in `TestToolFailureUnknownEffectsCountOnce`.

`TestGraphConformanceBrokerReplayRisk` is the failing-first reproducer. It
submits the same run identity and arguments to the real broker twice, modeling
recovery after loss in the post-effect/pre-completion window. The exact named
test observed `same broker operation executed 2 times across replay, want
exactly one`. This is not an inference from call counts inside a fake scheduler:
the probe's tool is invoked by the production broker function and measures the
external effect boundary directly.

G0 requires one brokered effect across injected crash/restart. Closing this
gap needs a durable idempotency/result boundary owned by the Service/tool host
(and conformance for both storage backends), which is durability/schema work
reserved for later tasks and explicitly outside Task 5. An in-memory counter,
an idempotent test lambda, or assuming that `tool.started` means the effect
happened would not distinguish the two crash sides and would manufacture a GO.

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

These APIs are sufficient for native scheduling, overlap, joins, interruption,
cancellation propagation, and opaque checkpoint resume. They are not an
exactly-once external-effect broker.

## Plan-vs-need decision

The plan allowed either a minimum native adapter or an evidence-backed NO-GO.
The user-visible safety contract takes precedence over demonstrating the other
five G0 observations in isolation. Because all six criteria are conjunctive
and effect safety is impossible to establish inside the authorized change
boundary, implementation stopped before creating a partial production path.

Alternatives rejected:

- Replaying the broker call with the same Workflow node: the broker exposes no
  durable operation key and calls the tool again.
- Treating a pre-effect Journal event as completion: it cannot prove whether
  the process crossed the external-effect boundary.
- Reusing the existing Journal as a dedupe ledger: `run_events` is unique only
  on `(run_id, seq)`. `tool_call_id` is payload data, and the child path neither
  queries it before execution nor has an atomic external-effect transaction.
- Reusing `SnapshotStore` or opaque checkpoint blobs: writing pending before
  execution leaves the same ambiguity; writing completed after execution
  leaves the same crash window.
- Using an idempotent test-only tool: it would prove the fixture, not arbitrary
  governed brokered effects.
- Adding an effect table/transactional outbox now: this broadens Task 5 into
  later schema and cross-backend work forbidden by the gate instructions.

## Scope and non-goals

No scheduler, child loop, graph lifecycle, schema, migration, RPC, UI, or
fallback runtime was added. One intentional RED broker replay reproducer was
added beside the existing broker tests. The Task 4 Service sentinel and its
failing integration test remain unchanged so the missing native production
path stays visible. The approval PIN owner files `internal/runtime/service.go`
and `internal/runtime/approval_test.go` are byte-identical to the Task 4 parent.
