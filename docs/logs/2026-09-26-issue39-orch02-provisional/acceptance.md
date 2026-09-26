# Issue 39 provisional acceptance status — 2026-09-27

**Disposition: PARTIAL — previously missing verification evidence is now produced (2026-09-27 pass); G4 remains BLOCKED.** ORCH-02–07 implementation is present in the worktree. Repository-wide Go tests, vet, builds, PostgreSQL-backed conformance, and a recorded real-browser pass at `127.0.0.1:3015` are now on record in [verification.md](verification.md). This still does not authorize merge, release, or Issue #39 closure: real model-path E2E and descendant resource accounting remain open, and the final review is the owner's.

| Requirement area | Worktree state | Acceptance status |
| --- | --- | --- |
| R1 Service/Eino-governed child execution | Native one-shot and continuable activation paths, parent-bounded tool selection, approval and cancellation are implemented; repository CI evidence and restart/cancellation browser checks now pass. | Budget/descendant accounting remains the open G1 item. |
| R2–R3 child Session, continuation, inspection and controls | Durable ChildSession bindings, same-origin Run reauthorization, post-completion scoped history, RPC and UI surfaces are implemented; PostgreSQL conformance and the real browser pass now cover persistence, mailbox admission and inspection. | Real model-path E2E remains the open G3 item. |
| R4–R5 finite DAG and native Eino Workflow | Descriptor validator, immutable revision storage, node admission, explicit dependency outputs and Workflow execution are implemented; both-backend conformance plus browser propose/start/cancel/restart checks now pass. | Integrated recovery/resource matrix remains open under G2. |
| R6 Run/Journal and host projection | Child/workflow lifecycle events, RPC projection, and RunInspector panels are implemented. | Restart/reload rehydration and fenced-recovery verified in the recorded browser pass. |
| R7 release evidence | Verification is recorded in [verification.md](verification.md), now including repository-wide tests, PostgreSQL conformance and a recorded browser pass. | Substantially produced; literal `just ci`, R13 accounting and real model-path E2E remain open. |
| R8 direct mailbox | Stable ChildSession addressing, direct sender authorization, per-recipient ordering/receipts, safe-point consumption, retry, parent model-input delivery, explicit `child_inbox`, and Run-scoped `reply_parent` keys are implemented. Limits: 128 admitted messages per recipient per ChildSession and 32 KiB per body. | PostgreSQL conformance and browser mailbox admission now pass; real model-path delivery remains unverified. Delivery remains at-least-once; no exactly-once effect claim. |
| R9 interruption/cancellation/close | Interrupt and Run cancellation are distinct; parent cancellation now propagates to ordinary child Runs and workflow descendants. ChildSession persists across activation interruption. A user-facing close operation is not offered. | D9 close and descendant disposition remain partially open. |
| R10 clean context | Child input is explicit task, admitted direct messages, and declared workflow outputs. | Focused tests pass; integrated prompt/authority parity remains under G1. |
| R11 tool permissions | Continuable children and workflow nodes use read-only tools, with the mailbox-only `reply_parent` exception. Existing explicit one-shot Service requests can select parent-authorized effectful tools under normal approval and isolated workspace. | Focused approval test passes; no general writable-child mode is accepted. |
| R12 parent model and narrowing | Child authority uses the current parent model and cannot widen the stored tool ceiling. | Focused tests pass; PostgreSQL conformance and restart rehydration now verified. |
| R13 resource and cost roll-up | Existing shared run-tree budget ledger and child/workflow accounting are wired into execution. | Full descendant budget/usage reconciliation across reauthorization and restart, durable slot/backpressure behavior and non-duplicating cost projection remain unverified. |
| R14 lifecycle modes | One-shot and continuable modes are explicit; workflow nodes are one-shot and non-addressable. | Focused tests, PostgreSQL conformance and browser lifecycle checks pass; G1 accounting remains open. |

## Delivery gates

| Gate | Status | Remaining evidence |
| --- | --- | --- |
| G0 technical feasibility | **EVIDENCE PRODUCED** | Repository-wide `go test ./...` (72 pkgs), `go vet ./...`, command builds, and PostgreSQL-backed conformance (CN-01..CN-33 + V14 in-place upgrade on Postgres 16) all pass. Literal `just ci` still needs the supported PowerShell environment; owner review confirms. |
| G1 child parity | **PARTIAL** | Child admission, continuable lifecycle, mailbox, interrupt, cancellation, restart rehydration and fenced-recovery are exercised in tests and the recorded browser pass. Descendant budget/usage reconciliation and accounting across reauthorization/restart remain unverified (R13). |
| G2 graph safety | **PARTIAL** | Both-backend admission covered by Postgres conformance; workflow propose/start/cancel and restart fencing exercised in the browser pass. The full integrated recovery/resource matrix remains open. |
| G3 product acceptance | **PARTIAL** | Real `:3015` host/browser scenarios, reload/restart, zh/en locale, and keyboard reachability verified and recorded. A real model path (actual child replies, node outputs, `reply_parent` delivery) needs a provider key — not available in this workspace. |
| G4 release | **BLOCKED** | `just ci` on the supported environment, owner E2E with a real provider key, and final release review. |

No merge, release or Issue #39 closure is authorized by this record. The owner's real E2E and release review remain required; this pass supplies the evidence that was previously missing.
