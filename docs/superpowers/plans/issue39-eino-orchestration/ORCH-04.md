# ORCH-04 — child follow-up, inspection and interruption

> **For implementers:** REQUIRED SUB-SKILL: `superpowers:executing-plans` or explicitly assigned `superpowers:subagent-driven-development`.

**Goal:** Let parent and user inspect, follow up and interrupt a task-scoped child through the same host authority and UI store.
**Architecture:** Extend current ChildController/RPC and RunInspector after ORCH-03 native execution; durable Session/Run state is the source for `wait` after restart.
**Tech Stack:** Go JSON-RPC, React/TypeScript, existing Face store and locales.
**Spec:** [architecture](../../specs/2026-09-23-issue39-eino-orchestration-design.md), R2/R3/R6/R8/R9/R14; [index](index.md) predecessors ORCH-03 and ORCH-02 mode contract.
**Review focus:** ID vs Session confusion, caller cancellation on wait, idempotent interrupt, hidden child Sessions, and compatible existing controls.

**Mailbox compatibility:** Message routes use stable Agent `SessionID` for the persistent recipient and `RunID` only for an activation. Host resolves sender identity and relationship authority; initial scope can restrict delivery to the direct parent/child pair, while the envelope remains extensible to a separately authorized peer relation under D5. `child/followup` starts a new Run; it is not delivery to a live child. D8 gates exposing live send/receive/ack controls until safe-point, retry, delivery-state and restart semantics are proven. Peer-to-peer swarm traffic remains unauthorized and deferred.

The compatibility floor is acceptance receipt only (stable message ID; no waiting for a reply) and activation-only interrupt (child Session, pending mail and descendants remain). A separately addressed reply and later activation are independent operations. Prove cancellation after acceptance, pending-mail restart and idle-child wake before claiming D8 complete.

Only continuable child modes are mailbox addresses. One-shot children expose their bounded task result/error and cannot be targeted after completion. The host/UI must label session identity and activation Run identity separately so `send_message`, interrupt, follow-up and close cannot accidentally target the wrong lifecycle.

## Files and interface

Modify `internal/rpc/{control,control_test}.go` (`ChildController`, `ChildRequest`, `ChildResult`), `internal/app/{worker,worker_test}.go`, `ui/src/lib/{api,api.test,store,store.test}.ts`, `ui/src/components/chat/RunInspector.tsx`, `ui/src/i18n/{en,zh}.ts` plus interaction tests if needed. Existing `ChildRun` is declared in `ui/src/lib/api.ts`; change `types.ts` only if the shared Run/Session contract actually changes. Existing `child/start,get,list,wait,cancel` remain. Proposed `child/followup` `{child_session_id, operation_id, text}` returns new child Run ID; proposed `child/interrupt` `{run_id}` stops only the current activation and leaves the ChildSession eligible for follow-up if its parent remains authorized. Scope reads to origin Session/authority; `child/get` remains Run-oriented. Do not redefine legacy `child/cancel` as a child-session close. Add a close action only after D9 specifies what it means for pending mail and descendants. Add `child/history` only if existing `session/messages` cannot provide scoped child transcript without revealing it in the main sidebar; decide from actual host API and Face contract, not convenience.

## Tasks

- [ ] Write RPC tests for invalid child Session, non-origin parent, terminal parent follow-up rejection, replayed operation ID, interruption during a turn while preserving the child Session, cancellation/close semantics as resolved by D9, pending mailbox outcomes, terminal interrupt idempotence and `wait` after process restart. Document backward-compatible `child/cancel` behavior.
- [ ] Preserve the mailbox address seam in types/storage by stable ChildSessionID. Before exposing message delivery, disposition D8 with the product owner and verify sender/recipient authority, ordering under concurrent sends, receipt/retry behavior, pending mail through interruption/restart, Eino safe-point consumption, expiry/backpressure and redaction. If not yet resolved, implement only status, follow-up as a new Run, and interrupt/cancel contracts; do not claim the earlier message candidate as implemented.
- [ ] Implement scoped host methods and Service wait-by-durable-Run. A live waiter may subscribe to Run events, but must re-read terminal status after subscription and on wake to avoid a lost-notification race; obey caller context cancellation. Return stable typed error codes; no raw policy or hidden prompt in child list.
- [ ] Wire API/types/store with only host-authoritative child state. Present task Session history, mask hint/provenance where permitted, follow-up composer, interrupt and status in RunInspector; keep start/cancel/get/list/wait controls. Use locale catalogs and accessible labels. Ensure sidebar does not show child Session as an unrelated normal conversation.
- [ ] Run `go test ./internal/rpc ./internal/app -run 'Child|Wait' -count=1`, `cd ui && pnpm test` (`vitest run` per current script), `just ci`, then manually exercise via `just dev` at `http://127.0.0.1:3015` including page reload and restart.

## Failure and rollback

If Host cannot safely scope transcript access, keep history UI gated and present status/results only; do not copy parent transcript into child. Do not change old method payloads in place. A retry must return the same activation or conflict. Record RPC request/response fixtures and UI screenshots/acceptance notes under release log; do not mark G3 passed without real backend smoke.
