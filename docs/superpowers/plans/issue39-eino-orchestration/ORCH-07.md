# ORCH-07 — workflow host and UI surface

> **For implementers:** REQUIRED SUB-SKILL: `superpowers:executing-plans` or explicitly assigned `superpowers:subagent-driven-development`.

**Goal:** Let an authorized parent submit a graph and a user inspect and cancel it with real host-derived node state.
**Architecture:** Control RPC projects ORCH-05/06 revision, Run and Journal into existing Face store/RunInspector; no second browser state machine.
**Tech Stack:** Go JSON-RPC, React/TypeScript, localization.
**Spec:** [architecture](../../specs/2026-09-23-issue39-eino-orchestration-design.md), R3/R4/R6; [index](index.md) predecessors ORCH-04 and ORCH-06.
**Review focus:** proposal/start errors actionable; status identity and permissions stable across page reload; no mock workflow or unsupported action shown.

## Files and interface

Modify `internal/rpc/{control,control_test}.go`, App control wiring, `ui/src/lib/{api,api.test,store,store.test}.ts`, `ui/src/components/chat/RunInspector.tsx`, `ui/src/i18n/{en,zh}.ts`, relevant component tests; include `types.ts` only for a shared Run/Session shape change. Proposed methods `workflow/propose` (pure validation of an inline descriptor; returns errors and canonical digest), `workflow/start` (operation ID plus the inline descriptor and digest; atomically persists the immutable revision and Run), `workflow/get`, `workflow/list`, `workflow/cancel`. `workflow/propose` persists neither revision nor Run; `start` revalidates the descriptor and parent authority at admission, rejecting digest mismatch. Revise names only with contract tests and design update. Read `docs/architecture/VIVY-FACE-PACK.md` for published UI contract; do not edit `ui/src/generated/**`.

## Tasks

- [ ] Add host tests: malformed/cyclic graph rejected before Run, replayed start same Run ID, changed payload conflict, unauthorized parent or stale revision rejected, graph status after restart from Journal/revision, cancellation while approval pending, no authority or raw checkpoint exposed.
- [ ] Implement typed RPC and App composition against Service graph methods. Return graph Run status with revision digest, node key, mapped child Run ID/status and bounded result/error; distinguish proposed, active, terminal. Sort nodes deterministically by descriptor order and keep event sequence for UI replay. Existing `child/*` methods remain unchanged.
- [ ] Extend Face store and API, show graph proposal validation, active graph and node list in RunInspector (or existing nearby panel if UI review establishes better fit). Link to child detail, show dependent waiting/failed/cancelled states accurately, offer cancel only while allowed; add accessible labels, loading/error/empty states and English/Chinese text.
- [ ] Run `go test ./internal/rpc ./internal/app -run 'Workflow|Child' -count=1`, UI typecheck/tests through `just ui-ci`, `just ci`, and `just dev` browser smoke on `http://127.0.0.1:3015` for two parallel nodes/join, reload, cancel and invalid graph. Capture request/response fixtures and real screenshots in release evidence.

## Failure and rollback

If graph projection lacks a durable node mapping, block UI instead of constructing it from process memory. Backend can keep graph execution gated while UI changes revert independently; keep historical graph read APIs compatible. Do not present optional #40 programmable tool/workflow authoring product as delivered. G3 passes only after real host/UI path and locales are checked.
