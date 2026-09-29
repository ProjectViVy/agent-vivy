# S11-D — Native child NodeExecutor and recovery

> **For agentic workers:** Execute each task with superpowers:executing-plans or subagent-driven-development; use test-driven-development for implementation and verification-before-completion before claiming a gate. Check boxes are tracking, not evidence of completion.

**Spec:** [detailed architecture](../../specs/2026-09-29-inofy-cutover-design.md), [issue #23](https://github.com/ProjectViVy/agent-vivy/issues/23), [package index](./README.md). Baselines are VIVY `3c4ed668` and INOFY `dbfebcec`; re-read current heads, AGENTS.md, and scoped instructions before implementation. All tasks below are **Planned**, with no implementation or passing-test claim. Record actual command/output and commit SHA in the iteration log.

**Global constraints:** VIVY owns Run/session/policy/budget/Journal/Core Storage and UI authorization. INOFY is the only executable graph engine. No old descriptor translator, engine selector, INOFY App service/database, or second agent loop. Keep Eino imports within VIVY `internal/runtime`/`internal/provider`; only runtime imports executable INOFY. Preserve normal chat/direct delegation and native child sessions. Add paired append-only SQLite/PostgreSQL migrations; do not rewrite 033. Do not edit generated UI assembly. Product UI belongs to a selected VIVY UI Module and Recipe.

**Goal:** Execute an INOFY Agent call as one governed VIVY child activation with safe reconciliation after interruption.

**Predecessor:** S11-C. **Unlocks:** S11-E. **Repository:** VIVY.

**Files:** `internal/runtime/workflow_service.go`, `child_oneshot.go`, `child_sessions.go`, `child_activation.go`, `child_recovery.go`; new `internal/runtime/inofy_executor.go` and focused tests. Inspect interfaces in INOFY `types.go`, `program.go`, `execution.go` at pinned revision.

**Contract:** Implement `NodeExecutor.Execute(ctx, NodeCall) (NodeReply,error)` for only trusted `vivy.child-task@1` ImplementationID. Validate Ref/path/config/input and active host authority; recheck tool ceiling before child admission. Use `NodeCall.OperationKey` (RunID/logical path) to derive deterministic `workflow_child_` ID; bind task/config/input digest to the operation. Repeated same effect joins/loads one native child, changed input conflicts. Return bounded output only from durable completed `ChildRunDetails`; explicitly label predecessor output untrusted in child task. Register non-replayable, max one effect attempt; do not invoke model again after unknown outcome. Map running/waiting/recovery_required into host projection; a node activation never owns the entire continuable session or mailbox. Cancellation stops new admissions and propagates to graph-owned active one-shot children.

- [ ] Add failing tests for parallel child fan-out/dependent synthesis with explicit output binding, stable child ID, duplicate operation key, changed digest, tool narrowing, depth, child terminal before result commit, crash before/after child admission, orphan model effect, cancel race and ordinary direct delegation/continuable session regression.
- [ ] Run `go test ./internal/runtime -run 'INOFY|Workflow|Child' -count=1`; record expected failures.
- [ ] Implement the adapter and recovery classifier. Reconcile durable child lifecycle plus checkpoint before replay; unresolved outcome becomes recovery_required, not success or fresh model invocation. Keep workflow node event terminal ownership with the committed INOFY step.
- [ ] Rerun focused tests with fault injection and `just ci`; report which real restart/cancel scenarios were covered.

**Review focus:** Child authority, effect-before-result crash, no double terminal, future interactive child boundary. **Acceptance:** G2 and child-effect portions of G5/G6; production still waits for S11-E.
