# Issue 39 provisional acceptance status — 2026-09-27

**Disposition: BLOCKED for acceptance and release.** ORCH-02–07 implementation is present in the worktree. This is implementation readiness only; it does not pass G0/G1 or the later integration/release gates.

| Requirement area | Worktree state | Acceptance status |
| --- | --- | --- |
| R1 Service/Eino-governed child execution | Native one-shot and continuable activation paths, parent-bounded tool selection, approval and cancellation are implemented; focused runtime/app tests pass. | G0/G1 blocked pending integrated parity, restart, budget and repository CI evidence. |
| R2–R3 child Session, continuation, inspection and controls | Durable ChildSession bindings, same-origin Run reauthorization, post-completion scoped history, RPC and UI surfaces are implemented; focused Go/UI tests pass. | G1/G3 blocked pending PostgreSQL and real host/browser E2E. |
| R4–R5 finite DAG and native Eino Workflow | Descriptor validator, immutable revision storage, node admission, explicit dependency outputs and Workflow execution are implemented; focused tests pass. | G0/G2 blocked pending both-backend and integrated recovery/cancellation acceptance. |
| R6 Run/Journal and host projection | Child/workflow lifecycle events, RPC projection, and RunInspector panels are implemented. | Integrated restart and real browser verification remain outstanding. |
| R7 release evidence | Focused verification is recorded in [verification.md](verification.md). | Not accepted; `just ci`, PostgreSQL, R1–R14 and browser/E2E evidence remain absent. |
| R8 direct mailbox | Stable ChildSession addressing, direct sender authorization, per-recipient ordering/receipts, safe-point consumption, retry, parent model-input delivery, explicit `child_inbox`, and Run-scoped `reply_parent` keys are implemented. Limits: 128 admitted messages per recipient per ChildSession and 32 KiB per body. | SQLite tests pass; PostgreSQL and real E2E remain unverified. Delivery remains at-least-once; no exactly-once effect claim. |
| R9 interruption/cancellation/close | Interrupt and Run cancellation are distinct; parent cancellation now propagates to ordinary child Runs and workflow descendants. ChildSession persists across activation interruption. A user-facing close operation is not offered. | D9 close and descendant disposition remain partially open. |
| R10 clean context | Child input is explicit task, admitted direct messages, and declared workflow outputs. | Focused tests pass; integrated prompt/authority parity remains under G1. |
| R11 tool permissions | Continuable children and workflow nodes use read-only tools, with the mailbox-only `reply_parent` exception. Existing explicit one-shot Service requests can select parent-authorized effectful tools under normal approval and isolated workspace. | Focused approval test passes; no general writable-child mode is accepted. |
| R12 parent model and narrowing | Child authority uses the current parent model and cannot widen the stored tool ceiling. | Focused tests pass; PostgreSQL/restart integration remains blocked. |
| R13 resource and cost roll-up | Existing shared run-tree budget ledger and child/workflow accounting are wired into execution. | Full descendant budget/usage reconciliation across reauthorization and restart, durable slot/backpressure behavior and non-duplicating cost projection remain unverified. |
| R14 lifecycle modes | One-shot and continuable modes are explicit; workflow nodes are one-shot and non-addressable. | Focused SQLite/runtime tests pass; PostgreSQL and G1 acceptance remain blocked. |

## Delivery gates

| Gate | Status | Remaining evidence |
| --- | --- | --- |
| G0 technical feasibility | **BLOCKED** | Repository CI and full required recovery proof on the supported environment; PostgreSQL-backed conformance. |
| G1 child parity | **BLOCKED** | Integrated authority, approval, budget, cancellation, restart and descendant accounting evidence. |
| G2 graph safety | **BLOCKED** | Both-backend admission and integrated workflow recovery/resource matrix. |
| G3 product acceptance | **BLOCKED** | Real `:3015` host/browser scenarios, reload/restart, accessibility and locale review. |
| G4 release | **BLOCKED** | `just ci`, accepted G0–G3, owner E2E and final release review. |

No merge, release or Issue #39 closure is authorized by this record. The owner will run real E2E after implementation is complete.
