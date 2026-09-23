# ORCH-04 — child follow-up, inspection and interruption

> **For implementers:** REQUIRED SUB-SKILL: `superpowers:executing-plans` or explicitly assigned `superpowers:subagent-driven-development`.

**Goal:** Let parent and user inspect, follow up and interrupt a task-scoped child through the same host authority and UI store.
**Architecture:** Extend current ChildController/RPC and RunInspector after ORCH-03 native execution; durable Session/Run state is the source for `wait` after restart.
**Tech Stack:** Go JSON-RPC, React/TypeScript, existing Face store and locales.
**Spec:** [architecture](../../specs/2026-09-23-issue39-eino-orchestration-design.md), R2/R3/R6; [index](index.md) predecessor ORCH-03.
**Review focus:** ID vs Session confusion, caller cancellation on wait, idempotent interrupt, hidden child Sessions, and compatible existing controls.

## Files and interface

Modify `internal/rpc/{control,control_test}.go` (`ChildController`, `ChildRequest`, `ChildResult`), `internal/app/{worker,worker_test}.go`, `ui/src/lib/{api,api.test,store,store.test}.ts`, `ui/src/components/chat/RunInspector.tsx`, `ui/src/i18n/{en,zh}.ts` plus interaction tests if needed. Existing `ChildRun` is declared in `ui/src/lib/api.ts`; change `types.ts` only if the shared Run/Session contract actually changes. Existing `child/start,get,list,wait,cancel` remain. Proposed `child/followup` `{child_session_id, operation_id, text}` returns new child Run ID; `child/interrupt` `{run_id}` returns durable child result. Scope reads to origin Session/authority; `child/get` remains Run-oriented. Add `child/history` only if existing `session/messages` cannot provide scoped child transcript without revealing it in the main sidebar; decide from actual host API and Face contract, not convenience.

## Tasks

- [ ] Write RPC tests for invalid child Session, non-origin parent, terminal parent follow-up rejection, replayed operation ID, cancellation of a pending approval, terminal interrupt idempotence and `wait` after process restart. Document backward-compatible `child/cancel` behavior.
- [ ] Implement scoped host methods and Service wait-by-durable-Run. A live waiter may subscribe to Run events, but must re-read terminal status after subscription and on wake to avoid a lost-notification race; obey caller context cancellation. Return stable typed error codes; no raw policy or hidden prompt in child list.
- [ ] Wire API/types/store with only host-authoritative child state. Present task Session history, mask hint/provenance where permitted, follow-up composer, interrupt and status in RunInspector; keep start/cancel/get/list/wait controls. Use locale catalogs and accessible labels. Ensure sidebar does not show child Session as an unrelated normal conversation.
- [ ] Run `go test ./internal/rpc ./internal/app -run 'Child|Wait' -count=1`, `cd ui && pnpm test` (`vitest run` per current script), `just ci`, then manually exercise via `just dev` at `http://127.0.0.1:3015` including page reload and restart.

## Failure and rollback

If Host cannot safely scope transcript access, keep history UI gated and present status/results only; do not copy parent transcript into child. Do not change old method payloads in place. A retry must return the same activation or conflict. Record RPC request/response fixtures and UI screenshots/acceptance notes under release log; do not mark G3 passed without real backend smoke.
