# S11-E — Sole production graph path and legacy removal

> **For agentic workers:** Execute each task with superpowers:executing-plans or subagent-driven-development; use test-driven-development for implementation and verification-before-completion before claiming a gate. Check boxes are tracking, not evidence of completion.

**Spec:** [detailed architecture](../../specs/2026-09-29-inofy-cutover-design.md), [issue #23](https://github.com/ProjectViVy/agent-vivy/issues/23), [package index](./README.md). Baselines are VIVY `3c4ed668` and INOFY `dbfebcec`; re-read current heads, AGENTS.md, and scoped instructions before implementation. All tasks below are **Planned**, with no implementation or passing-test claim. Record actual command/output and commit SHA in the iteration log.

**Global constraints:** VIVY owns Run/session/policy/budget/Journal/Core Storage and UI authorization. INOFY is the only executable graph engine. No old descriptor translator, engine selector, INOFY App service/database, or second agent loop. Keep Eino imports within VIVY `internal/runtime`/`internal/provider`; only runtime imports executable INOFY. Preserve normal chat/direct delegation and native child sessions. Add paired append-only SQLite/PostgreSQL migrations; do not rewrite 033. Do not edit generated UI assembly. Product UI belongs to a selected VIVY UI Module and Recipe.

**Goal:** Route every new VIVY task graph through INOFY, remove old executable DAG machinery, and prove the real task workflow.

**Predecessor:** S11-D. **Unlocks:** S11-F. **Repository:** VIVY.

**Files:** `internal/runtime/workflow.go`, `workflow_service.go`, `checkpoint.go`/`checkpointadapter.go` as applicable, `internal/orchestration/descriptor.go`, tool/RPC/UI callers and tests, `go.mod`/`go.sum`, iteration log. Locate all old graph compile paths and descriptor references by `rg` before deletion.

**Contract:** Pin INOFY at an immutable reviewed version/commit, compile the admitted normalized Definition with trusted catalog and ProgramMeta, run `Program.Run` with S11-C RunStore and S11-D NodeExecutor. The INOFY terminal commit is the only graph native terminal owner; remove the old `launchWorkflow` terminal emitter and `compose.NewWorkflow` execution route. Legacy discriminator rows stay historical and excluded from auto-recovery; no translation, selector, cross-engine resume or deletion of evidence. Native chat, direct children and existing sessions remain intact. Old first-party payloads fail clearly.

- [ ] Add failing real-path tests for workflow tool→admission→parallel nodes→dependent output→inspect/cancel; restart classification, invalid legacy row, terminal uniqueness, cancellation, event/inspection consistency. Include an assertion or source audit that no old graph compiler is production reachable.
- [ ] Run focused `go test ./internal/runtime ./internal/tools ./internal/rpc -run 'Workflow|INOFY' -count=1`; capture failure before route change.
- [ ] Switch the route atomically, remove obsolete descriptor/compiler/checkpoint-adapter code only where unused, adjust first-party UI contract, and keep one inspector backed by committed projection.
- [ ] Run `just ci`, SQLite/PostgreSQL upgrade/reopen/fault cases and VIVY split-UI real-path smoke required by AGENTS; record exact commands/results/skips in iteration log. Commit after G1–G8 evidence; if a backend is unavailable, mark gate blocked rather than claim completion.

**Review focus:** One engine and terminal owner, no stale call site, normal session retention, no unsafe binary rollback claim. **Acceptance:** G1–G8 core cutover; reusable product remains S11-F/G.
