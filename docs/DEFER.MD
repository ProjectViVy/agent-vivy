# AGENT-VIVY — Deferred, Superseded, and Declined Work

> Board for work deliberately not scheduled, moved out of `docs/TODO.md`.
> `docs/TODO.md` §0.1 remains the living open board; completed history is `docs/COMPLETE.MD`.
> Original IDs are retained so historical references stay stable. A deferral is not a start order.
> Moved on 2026-09-19.

---

## 1. Deferred

Deliberately not scheduled. Each row keeps the reason it waits and the record that would restart it.

| ID | Item | Status | Notes |
|---|---|---|---|
| CH-C8 | `vivy channel --name` child process in the same binary | DEFERRED | Memo Plan: `docs/plans/channel-epic/CH-C8.md`. Not a start order |
| CH-C9 | A2A / NeuroLink capability proposal | DEFERRED | Memo Plan: `docs/plans/channel-epic/CH-C9.md`. Not a start order |
| HITL-P1-7 | Generic edit + bulk approval | DEFERRED | Explicitly not P0 |
| MEM-1 | Memory / BML provider track | PARTIALLY LANDED — WIRING + LATER STAGES DEFERRED | BML restored as a standalone Go module `bml/` (record model, typed SQLite+FTS5 store, `MemoryHome` facade, migration adapters, `bml/provider.go` trait contract): PR #62 on `feat/memory`, log `docs/logs/2026-09-26-memory-bml/notes.md`. G0 contract freeze landed (`docs/architecture/VIVY-MEMORY-PROFILE.md`, `docs/architecture/VIVY-MEMORY-HOST-CONTRACTS.md`, `docs/research/2026-09-26-memory-upstream-pin.md`, `docs/research/2026-09-26-memory-g0-readiness.md`). Still deferred: runtime wiring (MEM-1A) and stages MEM-2..5 — stage rows in `docs/TODO.md` §0.1 and `docs/superpowers/plans/memory/index.md` |
| MEM-CAP | AutoDream / Evolution / RAG capabilities | DEFERRED | Split out of the former MEM-1 umbrella row on 2026-09-26: the BML library landing does not implement these broader capabilities; they still need their own capability proposal |
| P2-3 | QwenPaw filesystem-journal probe | DEFERRED | Out of V0; SR-4 was decided on 2026-09-02: keep QwenPaw externally obtained and recheck it against `qwenpaw-vendor-ruling.md` when the probe is proposed |
| P2-4 | Long-term section map for V3 | DEFERRED | |
| UI-TREE | Child-run tree visualization | DEFERRED | Harness GOAL-4/5 API exists; no tree UI |
| SBX-OS | OS-level process sandbox (bwrap / Seatbelt / Windows ACL) | DEFERRED | EINO currently provides only workspace paths + command allowlist + HTTP policy, not a DSH process sandbox |
| SBX-GLOB | Sandbox deny glob / editable auto_approve_tools | DEFERRED | The settings page exposes only presets and network policy; timeout remains in general settings |
| SBX-LIVE | Hot-swappable permission preset for an in-progress run | DEFERRED | The current turn pins the session policy; it takes effect on the next `turn/start` |
| ST-SUB-3 | Further split community plugins into nested submodules (optional) | DEFERRED | The initial tree is all in `vivy-studio`; split it after size/update policy stabilizes |
| WF-3 | Workflow follow-ups: trigger Port + single invocation entry, conditional edges/branch routing, node error policies (retry/skip), resume-after-interruption, structured (non-string) node IO, visual editor UI module, workflow nesting, version migration beyond schema v1, checkpoint-store mapping | DEFERRED | Umbrella for `DETAILED-DESIGN.md` §14 open items; none block WF-1/WF-2. Split into own rows when a slice is scheduled. |
| RB-L2-DEFER | Restore side: files/restore RPC + one-click session rollback UI (optionally include a Crush-style panel showing “which files this session changed”) | DEFERRED | Deferred by decision on 2026-09-01: the prototype MVP is not ready, so align with Crush first (the record side lands with the VC-3 tail); the restore side is a differentiation beyond Crush and can be revisited after MVP validation. The design is ready: rollback research §5.2/5.3/5.4, with O4 (archive scope) and O5 (restore governance) decisions to be made at kickoff |

## 2. Resolved-deferred

Closed by a recorded decision rather than by implementation; the decision is the evidence.

| ID | Item | Status | Notes |
|---|---|---|---|
| CH-C2-N2 | §8 slot cleanup: `run_id`/`task_id` on `InboundMessage`/`OutboundMessage` (written by Host) | RESOLVED-DEFERRED | 2026-08-30 (CH-C3): Delete/Reaction/HealthChecker/ListenHandler interfaces are present (v1 method set); the run_id/task_id slots are intentionally deferred because the pseudo-run design has no consumer, and SDK comments now describe the deferral. Add them only when C4/C8 actually needs them; do not rename existing fields |

## 3. Superseded

Replaced by a later breakdown. Kept as no-claim markers: do not claim these rows again.

| ID | Item | Status | Notes |
|---|---|---|---|
| CH-A | ChannelHost + telegram + dingtalk (coarse-grained) | SUPERSEDED | Split into CH-C1..C6 on 2026-08-30; see `docs/COMPLETE.MD` §0.2. Do not claim this row again |
| CH-B | feishu / qq / discord (coarse-grained) | SUPERSEDED | Split into CH-C7a/b/c on 2026-08-30; see `docs/COMPLETE.MD` §0.2. Do not claim this row again |

## 4. Won't do

Declined by an explicit owner decision. Do not reopen without a new decision.

| ID | Item | Status | Notes |
|---|---|---|---|
| ACP-1 | ACP / remote control **implementation** | WONT-DO | 2026-08-31 User decision: ACP will not be implemented (research §8.7) |

---

> Nothing here is deleted. When a deferral is lifted, move the row back to
> `docs/TODO.md` §0.1 with its new status and the new evidence.
