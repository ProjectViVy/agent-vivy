# S11-C — Core Storage transaction and INOFY RunStore

> **For agentic workers:** Execute each task with superpowers:executing-plans or subagent-driven-development; use test-driven-development for implementation and verification-before-completion before claiming a gate. Check boxes are tracking, not evidence of completion.

**Spec:** [detailed architecture](../../specs/2026-09-29-inofy-cutover-design.md), [issue #23](https://github.com/ProjectViVy/agent-vivy/issues/23), [package index](./README.md). Baselines are VIVY `3c4ed668` and INOFY `dbfebcec`; re-read current heads, AGENTS.md, and scoped instructions before implementation. All tasks below are **Planned**, with no implementation or passing-test claim. Record actual command/output and commit SHA in the iteration log.

**Global constraints:** VIVY owns Run/session/policy/budget/Journal/Core Storage and UI authorization. INOFY is the only executable graph engine. No old descriptor translator, engine selector, INOFY App service/database, or second agent loop. Keep Eino imports within VIVY `internal/runtime`/`internal/provider`; only runtime imports executable INOFY. Preserve normal chat/direct delegation and native child sessions. Add paired append-only SQLite/PostgreSQL migrations; do not rewrite 033. Do not edit generated UI assembly. Product UI belongs to a selected VIVY UI Module and Recipe.

**Goal:** Make each INOFY commit an atomic, idempotent VIVY workflow step in both database drivers.

**Predecessor:** S11-B. **Unlocks:** S11-D. **Repository:** VIVY.

**Files:** `internal/storage/contracts.go`, `workflow_revisions.go`, `internal/storage/{sqlite,postgres}/` workflow/Journal implementations and tests, paired new `migrations/{sqlite,postgres}/` numbered migration after current head; `internal/runtime/inofy_store.go` adapter and tests. Inspect actual layout/migration number before writing. Keep storage contracts host typed.

**Contract:** Add a narrow `CommitWorkflowStep(ctx, WorkflowStepCommit) (WorkflowStepReceipt,error)` / load operation (names proposed). One SQL transaction checks host binding, expected state, writer epoch, terminal invariant and unique `(run_id,commit_id)` content digest; duplicates return the same receipt, conflicts reject. Append contiguous redacted Journal events, update projection/result/checkpoint references and native Run status exactly once. Stage and verify immutable content-addressed workflow-scoped blobs before SQL; a failed SQL transaction leaves unreachable blobs, never a visible missing reference. Add nullable ProgramMeta/input/limits/authority identity columns to immutable revision admission. Storage discriminator 1=legacy, 2=INOFY. New rows require complete identity fields. Runtime adapter implements `RunStore.Commit/Load`, including INOFY's initial empty→admitted transition against an already accepted native Run at epoch 1, without a second run.started.

- [ ] Write failing SQLite and PostgreSQL tests for fresh admission+initial Commit, duplicate/conflicting CommitID, stale epoch, state/terminal conflict, contiguous event sequence, failed Journal transaction, blob-stage failure, orphan blob, missing/corrupt referenced blob, restart/load, fresh install and upgrade from migration 033.
- [ ] Run focused `go test ./internal/storage/... ./internal/runtime -run 'WorkflowStep|INOFYStore' -count=1`; capture initial failure and ensure both driver tests are actually exercised, not silently skipped.
- [ ] Add paired append-only migrations and transaction code; keep Journal as durable evidence and projection/receipt rows as transaction-local indexes. Map INOFY structures only inside runtime.
- [ ] Re-run fault and parity tests, then `just ci`; record actual PostgreSQL availability and skips. No route switch until both drivers and initial transition pass.

**Review focus:** Race/receipt semantics, no separate INOFY database, redacted events versus protected results, compatible ProgramMeta on load. **Acceptance:** G4 plus storage portions of G5/G6.
