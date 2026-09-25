# ORCH-02 — durable task conversation and admission

> **For implementers:** REQUIRED SUB-SKILL: `superpowers:executing-plans` or explicitly assigned `superpowers:subagent-driven-development`.

**Goal:** Define and persist compatible child lifecycle modes; only explicitly continuable children receive a task-scoped durable ChildSession with multiple activation Runs and exact lineage.
**Architecture:** Host-owned binding and idempotent operation key in both SQL backends; Service creates Session/Run under parent authority; Journal is the observable lifecycle. Implement the approved D14 split: one-shot by default for legacy synchronous agent and DAG nodes, continuable only when explicit.
**Tech Stack:** Go, SQLite/Postgres migrations, existing storage conformance.
**Spec:** [architecture](../../specs/2026-09-23-issue39-eino-orchestration-design.md), R1/R2/R6/R8/R14; [index](index.md) owns predecessor ORCH-01 and status.
**Review focus:** no child orphan on concurrent parent deletion, no duplicated admission after retry, no old same-Session transcript split retroactively.

## Files and contracts

Persist ChildSessionID, immutable origin parent Run/Session IDs, current authorizer Run ID, activation Run ID, original authority ceiling digest and operation key. Admission is idempotent; conflicting payload reuse fails. A new active Run in the same original parent Session can authorize continuation after origin Run termination. Effective authority is current authority intersected with original ceiling; deleting the original parent Session fences continuation. Legacy synchronous agent and DAG nodes default one-shot/non-addressable; only explicit continuable mode creates ChildSession.

Continuable ChildSessionID is the required stable mailbox address. Message identity, idempotency key, sequence and receipt never key to activation Run. Implement durable direct parent-child messaging, ordered lifecycle, safe-point consumption, durable receipt/cursor, retry/restart and at-least-once delivery. No exactly-once effect claim. One-shot children have no durable address.

## Tasks

- [ ] Encode approved D14: modes coexist; legacy synchronous agent and DAG nodes default one-shot/non-addressable. Only explicit continuable mode gets ChildSession and accepts follow-up/mail. Encode D4 reauthorization and deletion fence.
- [ ] Inspect current `domain`, SessionStore/RunStore, admission validation, `Service.DeleteSession`, run-tree migration 005 and last migration; specify exact transaction and uniqueness strategy before changing schema. Test two concurrent identical requests returning one child binding/Run and conflicting payload returning an explicit conflict; rejection must occur before any model/tool side effect.
- [ ] Add paired append-only SQL migrations, store implementation and conformance for binding, durable ordered direct mailbox, lifecycle, receipt/cursor/retry, follow-up and deletion. Delivery is at-least-once. Both SQL backends require conformance; deferred PostgreSQL execution is unverified, not passed.
- [ ] Implement Service durable admission and authority intersection. Reauthorization requires a new active Run in same original parent Session, even after origin Run termination. Deletion fences. Context is explicit task, admitted direct messages and approved dependency outputs only. Parent model only; selected tools may narrow immutable ceiling.
- [ ] Decide and test migration compatibility for historical `RunKindChild` rows sharing parent Session: list/read historical rows as before; new binding applies only to new children. Validate safe handling if an old child ID appears in a follow-up request.
- [ ] Run `go test ./internal/storage/... ./internal/runtime/... -run 'Child|DeleteSession|Admission' -count=1`; then `just ci`. PostgreSQL execution is owner-deferred to a later dedicated iteration; record it as unverified, not passed, and retain it as a required acceptance gate.

## Failure, release and rollback

Persist binding/Session/Run as one logical admission or use a compensating transaction proven under crash/deletion; never expose a child ID while its authoritative record is absent. Rollback is feature-disabled reads of new rows plus append-only corrective migration, never delete a shipped migration. Evidence: SQL schema pair, conformance logs, Run/Journal trace, and deletion race result in release log. Stop ORCH-03 if child authority rehydration or Session isolation cannot be proven.
