# ORCH-04 — child follow-up, inspection and interruption

> **For implementers:** REQUIRED SUB-SKILL: `superpowers:executing-plans` or explicitly assigned `superpowers:subagent-driven-development`.

**Goal:** Let parent and user inspect, follow up and interrupt a task-scoped child through the same host authority and UI store.
**Architecture:** Extend current ChildController/RPC and RunInspector after ORCH-03 native execution; durable Session/Run state is the source for `wait` after restart.
**Tech Stack:** Go JSON-RPC, React/TypeScript, existing Face store and locales.
**Spec:** [architecture](../../specs/2026-09-23-issue39-eino-orchestration-design.md), R2/R3/R6/R8/R9/R14; [index](index.md) predecessors ORCH-03 and ORCH-02 mode contract.
**Review focus:** ID vs Session confusion, caller cancellation on wait, idempotent interrupt, hidden child Sessions, and compatible existing controls.

**Required mailbox:** Direct parent-child messages route to stable `ChildSessionID`, never activation `RunID`. Host derives sender identity and authorizes only the direct relationship; it persists stable IDs, idempotent admission, sequence/order and lifecycle before ack. Consume at a verified safe point with durable cursor/receipt. Retry/restart can redeliver, so delivery is at-least-once, not exactly-once effects. Follow-up starts a new Run. Peer/swarm remains deferred.

Acceptance acknowledges durable admission only, returning stable message ID and no reply. Cancellation after admission does not retract it; idempotent retry returns the same ID. Interrupt stops activation and preserves ChildSession/pending mail. A separately addressed reply and later activation are distinct.

Only explicit continuable mode has stable ChildSession and follow-up/mail. Legacy synchronous agent and DAG nodes default one-shot/non-addressable, exposing bounded result/error only. Host/UI labels ChildSession and activation Run separately. A new active authorizer Run in the same original parent Session can reauthorize after origin Run termination, intersected with original authority ceiling; deletion fences.

## Files and interface

Modify `internal/rpc/{control,control_test}.go` (`ChildController`, `ChildRequest`, `ChildResult`), `internal/app/{worker,worker_test}.go`, `ui/src/lib/{api,api.test,store,store.test}.ts`, `ui/src/components/chat/RunInspector.tsx`, `ui/src/i18n/{en,zh}.ts` plus interaction tests if needed. Existing `ChildRun` is declared in `ui/src/lib/api.ts`; change `types.ts` only if the shared Run/Session contract actually changes. Existing `child/start,get,list,wait,cancel` remain. Proposed `child/followup` `{child_session_id, operation_id, text}` returns new child Run ID; proposed `child/interrupt` `{run_id}` stops only the current activation and leaves the ChildSession eligible for follow-up if its parent remains authorized. Scope reads to origin Session/authority; `child/get` remains Run-oriented. Do not redefine legacy `child/cancel` as a child-session close. Add a close action only after D9 specifies what it means for pending mail and descendants. Add `child/history` only if existing `session/messages` cannot provide scoped child transcript without revealing it in the main sidebar; decide from actual host API and Face contract, not convenience.

## Tasks

- [ ] Write RPC tests for invalid child Session, non-origin parent, terminal parent follow-up rejection, replayed operation ID, interruption during a turn while preserving the child Session, cancellation/close semantics as resolved by D9, pending mailbox outcomes, terminal interrupt idempotence and `wait` after process restart. Document backward-compatible `child/cancel` behavior.
- [ ] Implement required D8 mailbox: direct relationship authorization, stable message/idempotency IDs, recipient order, admission ack, lifecycle, safe-point consumption, durable receipt/cursor, expiry/backpressure/redaction and recovery. Duplicate delivery is at-least-once; never claim exactly-once effects.
- [ ] Implement scoped host methods and Service wait-by-durable-Run. Revalidate authorizer Run in same origin parent Session and intersect with original ceiling; deletion fences continuation/mail. Re-read terminal status on wake; no raw policy/transcript/hidden prompt in child list.
- [ ] Wire API/types/store with only host-authoritative child state. Present task Session history, mask hint/provenance where permitted, follow-up composer, interrupt and status in RunInspector; keep start/cancel/get/list/wait controls. Use locale catalogs and accessible labels. Ensure sidebar does not show child Session as an unrelated normal conversation.
- [ ] Run `go test ./internal/rpc ./internal/app -run 'Child|Wait' -count=1`, `cd ui && pnpm test` (`vitest run` per current script), `just ci`, then manually exercise via `just dev` at `http://127.0.0.1:3015` including page reload and restart.

## Failure and rollback

If Host cannot safely scope transcript access, keep history UI gated and present status/results only; do not copy parent transcript into child. Do not change old method payloads in place. A retry must return the same activation or conflict. Record RPC request/response fixtures and UI screenshots/acceptance notes under release log; do not mark G3 passed without real backend smoke.
