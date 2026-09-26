# ORCH-02 — durable task conversation and admission

> **For implementers:** REQUIRED SUB-SKILL: `superpowers:executing-plans` or explicitly assigned `superpowers:subagent-driven-development`.

**Goal:** Define and persist compatible child lifecycle modes; only explicitly continuable children receive a task-scoped durable ChildSession with multiple activation Runs and exact lineage.
**Architecture:** Host-owned binding and idempotent operation key in both SQL backends; Service creates Session/Run under parent authority; Journal is the observable lifecycle. Implement the approved D14 split: one-shot by default for legacy synchronous agent and DAG nodes, continuable only when explicit.
**Tech Stack:** Go, SQLite/Postgres migrations, existing storage conformance.
**Spec:** [architecture](../../specs/2026-09-23-issue39-eino-orchestration-design.md), R1/R2/R6/R8/R14; [index](index.md) owns predecessor ORCH-01 and status.
**Review focus:** no child orphan on concurrent parent deletion, no duplicated admission after retry, no old same-Session transcript split retroactively.

## Foundation handoff

ORCH-01 owns the minimum D15 operation/claim/result boundary and proof-node admission before G0. This Story consumes that proven seam; it is not a prerequisite for the proof and must not create a second tool-effect ledger. Extend only the child conversation, lineage and mailbox contracts below.

## Files and contracts

Persist ChildSessionID, immutable origin parent Run/Session IDs, current authorizer Run ID, activation Run ID, original authority ceiling digest and operation key. Admission is idempotent; conflicting payload reuse fails. A new active Run in the same original parent Session can authorize continuation after origin Run termination. Effective authority is current authority intersected with original ceiling; deleting the original parent Session fences continuation. Legacy synchronous agent and DAG nodes default one-shot/non-addressable; only explicit continuable mode creates ChildSession.

Continuable ChildSessionID is the required stable mailbox address. Message identity, sender-scoped idempotency key, recipient-inbox sequence and receipt never key to activation Run. Implement durable direct parent-child messaging, ordered lifecycle, safe-point consumption, durable receipt/cursor, retry/restart and at-least-once delivery. No exactly-once effect claim. One-shot children have no durable address.

### Storage transaction contract

- Add a persisted `child_mode` to new child Run rows. Empty mode normalizes to `one-shot`; only `continuable` creates an addressable ChildSession. Historical `RunKindChild` rows remain one-shot and retain their existing Session.
- Store one continuation binding per `(origin_parent_session_id, operation_key)`, with the stable ChildSessionID, immutable origin parent Run/Session, current authorizer Run, request digest, canonical authority-ceiling JSON and its digest, activation operation key/digest/tool set, and lifecycle. Record each activation operation in a child-scoped table so same-key/same-digest retries return the original activation Run and changed payload reuse returns `ErrChildAdmissionConflict`.
- `CommitChildAdmission` is the narrow atomic boundary: insert child Session, binding, initial user task message, child activation Run, prompt snapshot (when configured), and `run.started` Journal event in one SQL transaction. Return the committed IDs and event; the caller publishes only after commit. Never call a model or tool before this method succeeds.
- Under that transaction, lock the origin parent Session and authorizer Run. Require the authorizer to be an active Run in the same origin parent Session. PostgreSQL uses row locks; SQLite's single-connection transaction serializes the write. Deletion locks the same Session, recursively removes its continuable child Sessions, and fences later admissions. A failed/racing transaction creates no addressable child.
- Mailbox keys are `(child_session_id, sender_session_id, idempotency_key)`; each recipient inbox has its own sequence and cursor, allocated/advanced in the same transaction as message admission/receipt. Per-recipient order avoids gaps when traffic flows both ways; no cross-direction total ordering is promised. Messages are direct-only. A durable receipt is scoped to message and consuming Run. Retry/restart can redeliver an unconsumed message; acknowledgement means admitted, never exactly-once effect. Closing a ChildSession expires pending messages and rejects new mail.
- Keep historical Run rows sharing a parent Session readable. Binding lookup is used only for newly admitted continuable ChildSessions; a historical child ID cannot be treated as a mailbox address.

## Provisional implementation status (2026-09-26)

Owner-directed development is proceeding while G0 is BLOCKED. This status records code in the current worktree; it is not G0/G1 acceptance or release approval.

- Implemented locally: explicit child mode migration/backfill, paired SQLite/PostgreSQL binding and activation tables, canonical policy/sandbox/approval/tool ceiling persistence with digest verification, atomic child Session/message/Run/prompt/`run.started` admission, same-origin reauthorization, per-activation operation idempotency, Service admission methods, tool-set intersection, and recursive deletion fencing. The durable mailbox is bidirectional with recipient-scoped ordering/cursors; Service derives direct participants from the active parent/child Run and supports idempotent send, scoped pending reads and durable receipts. Native continuable activation now runs through Service/Eino, records child lifecycle, consumes admitted mail at activation safe points, and fails/retries receipts durably. Continuable children can send direct replies through `reply_parent`; each recipient is limited to 128 admitted messages per ChildSession and each body to 32 KiB.
- Reauthorization intersects current read-only tools with the immutable original tool ceiling. The current policy hash/profile and sandbox/approval settings must match the original snapshot; changed settings fail closed until a tested policy-intersection implementation replaces this conservative rule.
- SQLite backend conformance and focused runtime admission/deletion tests pass. PostgreSQL package compiles, but DSN-backed conformance was skipped and remains unverified.
- Exact commands and outcomes are recorded in the [provisional verification log](../../../logs/2026-09-26-issue39-orch02-provisional/verification.md).
- Existing synchronous worker children remain one-shot by default. Explicit one-shot requests may select only tools in the active parent authority ceiling; effectful calls remain under the existing Service approval path. Workflow and default child tool sets remain read-only.
- Focused SQLite storage/runtime/app/tool tests pass. PostgreSQL package compilation succeeds, but DSN-backed database behavior is unverified. The shared Service run-tree budget is wired into child activation; integrated descendant usage, restart/backpressure acceptance, G0/G1, `just ci`, browser/E2E and release remain blocked. No gate is waived.

## Tasks

- [ ] Encode approved D14: modes coexist; legacy synchronous agent and DAG nodes default one-shot/non-addressable. Only explicit continuable mode gets ChildSession and accepts follow-up/mail. Encode D4 reauthorization and deletion fence.
- [ ] Inspect current `domain`, SessionStore/RunStore, admission validation, `Service.DeleteSession`, run-tree migration 005 and last migration; specify exact transaction and uniqueness strategy before changing schema. Test two concurrent identical requests returning one child binding/Run and conflicting payload returning an explicit conflict; rejection must occur before any model/tool side effect.
- [ ] Add paired append-only SQL migrations, store implementation and conformance for binding, durable ordered direct mailbox, lifecycle, receipt/cursor/retry, follow-up and deletion. Delivery is at-least-once. Both SQL backends require conformance; deferred PostgreSQL execution is unverified, not passed.
- [ ] Implement Service durable admission and authority intersection. Reauthorization requires a new active Run in same original parent Session, even after origin Run termination. Deletion fences. Context is explicit task, admitted direct messages and approved dependency outputs only. Parent model only; selected tools may narrow immutable ceiling.
- [ ] Decide and test migration compatibility for historical `RunKindChild` rows sharing parent Session: list/read historical rows as before; new binding applies only to new children. Validate safe handling if an old child ID appears in a follow-up request.
- [ ] Run `go test ./internal/storage/... ./internal/runtime/... -run 'Child|DeleteSession|Admission' -count=1`; then `just ci`. PostgreSQL execution is owner-deferred to a later dedicated iteration; record it as unverified, not passed, and retain it as a required acceptance gate.

## Failure, release and rollback

Persist binding/Session/Run as one logical admission or use a compensating transaction proven under crash/deletion; never expose a child ID while its authoritative record is absent. Rollback is feature-disabled reads of new rows plus append-only corrective migration, never delete a shipped migration. Evidence: SQL schema pair, conformance logs, Run/Journal trace, and deletion race result in release log. Stop ORCH-03 if child authority rehydration or Session isolation cannot be proven.
