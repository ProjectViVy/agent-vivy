# ORCH-02 — durable task conversation and admission

> **For implementers:** REQUIRED SUB-SKILL: `superpowers:executing-plans` or explicitly assigned `superpowers:subagent-driven-development`.

**Goal:** Define and persist compatible child lifecycle modes; mailbox-addressable continuable children receive a task-scoped durable Session with multiple activation Runs and exact lineage.
**Architecture:** Host-owned binding and idempotent operation key in both SQL backends; Service creates Session/Run under parent authority; Journal is the observable lifecycle. D14 must settle which delegation paths are one-shot versus continuable before the schema fixes their identity.
**Tech Stack:** Go, SQLite/Postgres migrations, existing storage conformance.
**Spec:** [architecture](../../specs/2026-09-23-issue39-eino-orchestration-design.md), R1/R2/R6/R8/R14; [index](index.md) owns predecessor ORCH-01 and status.
**Review focus:** no child orphan on concurrent parent deletion, no duplicated admission after retry, no old same-Session transcript split retroactively.

## Files and contracts

Modify `internal/domain/{run,session}.go` **after confirming actual filenames** with `rg --files internal/domain`; `internal/storage/contracts.go`, `internal/storage/{sqlite,postgres}/{runs,sessions}.go` and their conformance tests; `internal/runtime/service.go` and its tests. Add `internal/storage/migrations/{sqlite,postgres}/NNN_child_sessions.sql` as the **same next unused number** in each backend after rebase (baseline last is 025). Optional NEW `internal/storage/{sqlite,postgres}/child_sessions.go` only for cohesive SQL, no parallel source of truth. Define a persistent binding with child session ID, origin parent Run/Session IDs, current Run ID, parent authority digest and operation key; store unique `(origin_parent_run_id, operation_id)` plus payload digest. Child Session metadata must distinguish it from ordinary sidebar Sessions and bind cascading deletion to origin Session. Use the existing Run `ParentID`, `RootID`, `Depth` fields; a follow-up Run retains original authority lineage instead of making the prior child Run its parent.

Treat continuable ChildSessionID as the stable future mailbox address; do not key message identity, sequence or pending receipt to current RunID. A one-shot child has no durable mailbox address and cannot later appear continuable by accident. Mailbox message storage and delivery states are not part of this Story unless D8 is resolved.

## Tasks

- [ ] Resolve the one-shot versus continuable mode boundary in D14 before schema work: map current `agentToolRef.StartAgentTask` and `child/start` to intended lifecycle, define whether a one-shot result gets a retained Session, and specify which mode accepts follow-up/mail. Preserve the existing one-shot UX unless an explicit product decision changes it.
- [ ] Inspect current `domain`, SessionStore/RunStore, admission validation, `Service.DeleteSession`, run-tree migration 005 and last migration; specify exact transaction and uniqueness strategy before changing schema. Test two concurrent identical requests returning one child binding/Run and conflicting payload returning an explicit conflict; rejection must occur before any model/tool side effect.
- [ ] Add paired append-only SQL migrations, query/store implementations and cross-backend conformance for create/read/list/follow-up linkage and delete. Add test: origin Session deleted concurrently with child creation leaves no Session, Run, checkpoint or dangling binding. No service-internal map is authoritative.
- [ ] Implement Service-side durable admission and parent-derived authority capture with existing caps (depth 4, concurrent children 4, turns 8 unless reviewed). Child Session task text and mask snapshot are immutable per Run; follow-up writes to same child Session but new Run. New activations require live parent authorization for this cut. Reject a child whose tool/profile/workspace broadens parent's rights.
- [ ] Decide and test migration compatibility for historical `RunKindChild` rows sharing parent Session: list/read historical rows as before; new binding applies only to new children. Validate safe handling if an old child ID appears in a follow-up request.
- [ ] Run `go test ./internal/storage/... ./internal/runtime/... -run 'Child|DeleteSession|Admission' -count=1`; then `just ci`. Check Postgres in the actual conformance environment, record any skipped backend as blocked.

## Failure, release and rollback

Persist binding/Session/Run as one logical admission or use a compensating transaction proven under crash/deletion; never expose a child ID while its authoritative record is absent. Rollback is feature-disabled reads of new rows plus append-only corrective migration, never delete a shipped migration. Evidence: SQL schema pair, conformance logs, Run/Journal trace, and deletion race result in release log. Stop ORCH-03 if child authority rehydration or Session isolation cannot be proven.
