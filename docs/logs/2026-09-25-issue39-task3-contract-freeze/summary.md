# Issue #39 Task 3: contract freeze

## Scope

Aligned the Issue #39 architecture, canonical Story index and ORCH-01 through ORCH-08 around approved D4, D8, D10, D12 and D14 contracts. This is documentation only. It does not implement production behavior, change Story status, or release work past ORCH-01.

Frozen decisions include post-origin-run reauthorization by a new active Run in the same parent Session, intersected with the immutable original authority ceiling; parent Session deletion fences continuation. Direct parent-child durable messaging is required, ordered, idempotently admitted and consumed safely with at-least-once delivery. Context is clean, model is parent-only, and legacy synchronous/DAG child modes are one-shot and non-addressable unless continuable mode is explicitly selected.

Pinned Eino v0.9.13 Workflow contracts now use `Workflow.End()`, `WorkflowNode.AddInput` and `AddDependency`; Host owns finite DAG bounds. Unreachable or unconsumed nodes are rejected. The index remains the sole Story-state DAG and ORCH-01 remains the only execution-ready Story. G0/G1 are not waived.

## Explicitly not done

No Go, UI, schema, migration, runtime, test, build, configuration or product behavior changes. PostgreSQL conformance execution is owner-deferred to a later dedicated iteration, remains unverified and is not a pass.
