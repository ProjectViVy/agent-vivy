# S11-B — VIVY definition admission and tool contract

> **For agentic workers:** Execute each task with superpowers:executing-plans or subagent-driven-development; use test-driven-development for implementation and verification-before-completion before claiming a gate. Check boxes are tracking, not evidence of completion.

**Spec:** [detailed architecture](../../specs/2026-09-29-inofy-cutover-design.md), [issue #23](https://github.com/ProjectViVy/agent-vivy/issues/23), [package index](./README.md). Baselines are VIVY `3c4ed668` and INOFY `dbfebcec`; re-read current heads, AGENTS.md, and scoped instructions before implementation. All tasks below are **Planned**, with no implementation or passing-test claim. Record actual command/output and commit SHA in the iteration log.

**Global constraints:** VIVY owns Run/session/policy/budget/Journal/Core Storage and UI authorization. INOFY is the only executable graph engine. No old descriptor translator, engine selector, INOFY App service/database, or second agent loop. Keep Eino imports within VIVY `internal/runtime`/`internal/provider`; only runtime imports executable INOFY. Preserve normal chat/direct delegation and native child sessions. Add paired append-only SQLite/PostgreSQL migrations; do not rewrite 033. Do not edit generated UI assembly. Product UI belongs to a selected VIVY UI Module and Recipe.

**Goal:** Admit only a normalized INOFY Definition for new task graphs, retaining VIVY's existing authorization and bounded read-only child behavior.

**Predecessor:** S11-A. **Unlocks:** S11-C. **Repository:** VIVY.

**Files:** `internal/tools/workflow.go`, `internal/rpc/control.go`, `internal/runtime/workflow_service.go`, `internal/orchestration/descriptor.go`, `internal/domain/workflow.go`, `ui/src/lib/workflow-api.ts`, focused existing tests. Inspect current call sites with `rg 'WorkflowRequest|Descriptor|ProposeWorkflow|StartWorkflow'`. The old descriptor file is removed at S11-E after all callers migrate.

**Contract:** The existing `workflow` tool and propose/start RPC receive `inofy.workflow/v1` Definition, not an integer-version DAG. Strict INOFY decoding, normalization, semantic validation and a frozen trusted catalog precede host authorization; the supported node is initially `vivy.child-task@1`. Bind parent/root Run, session, Generation, policy snapshot, operation key, input, tool ceiling, depth and read-only child scope to the admitted Run. Preserve 12 nodes/24 edges/four outputs and current byte/active-child ceilings unless current code proves a narrower bound. Use INOFY's canonical schema accessor for topology and project the host subset; never trust model-side schema alone. Persist normalized bytes and digests under the existing immutable admission transaction with an explicit new-format discriminator. S11-C adds execution identity columns atomically before production routing.

- [ ] Add failing tests for valid parallel/dependent Definition, legacy descriptor rejection, unknown node/type/config, oversize graph/input/output, binding error, duplicate operation key with different content, policy/tool widening, depth and generation mismatch. Include tool JSON schema and RPC/UI caller conformance.
- [ ] Run `go test ./internal/tools ./internal/rpc ./internal/runtime -run 'Workflow|INOFY' -count=1`; capture expected failures. Adjust test names to repository conventions.
- [ ] Update the tool/RPC/first-party payload together and use the frozen trusted catalog. Keep execution behind non-production path until durable adapter S11-C/D exists; reject attempts to run an unimplemented path explicitly.
- [ ] Rerun focused tests; commit only once old payload cannot enter a new admission and ordinary delegation tests still pass.

**Review focus:** Schema truth, one admission owner, no exposure of switch/repeat/wait merely because INOFY supports them. **Acceptance:** G1; admission code is not a claim of production cutover.
