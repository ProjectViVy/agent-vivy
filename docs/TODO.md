# AGENT-VIVY V0 — Project TODO Board

> Status: living board. Only currently open work remains.
> Completed history: `docs/COMPLETE.MD`.
> Deferred, superseded, and declined work: `docs/DEFER.MD`.
> Acceptance and release records: `docs/logs/`.
> Milestones map to PRD §11 (M0–M4). Acceptance anchors cite PRD FR/AS/D ids.
> Architecture reference: `IMPLEMENTATION-PLAN.md`.
> Updated: 2026-09-26
> Development environment: `docs/architecture/VIVY-STUDIO.md`. Studio is the first-party daily IDE; other authorized tools work directly in this repository with their own capabilities.

---

## 0. Start posture (read first)

Per `GO-NOGO-PREFLIGHT.md` §8(b), V0 starts with the soft requirements below
accepted as **RISK ACCEPTED** with monitors, rather than closed first:

| Item | Posture | Monitor |
|---|---|---|
| SR-1 ADR baseline missing (D-035) | CLOSED | `docs/AGENT-VIVY-ARCHITECTURE-V0.md` authored 2026-08-08 |
| SR-2 Eino §6 claims unverified (D-034) | CLOSED | Verified by task **A1** — see `eino-capability-verify.md` (2026-08-07) |
| P0-3 PRD v0.5 final confirmation | RISK ACCEPTED | User sign-off tracked here; treat v0.5 as final until told otherwise |
| P0-4 SR acceptance posture | RESOLVED | This table is the acceptance record |

Gate rule: **A1 must pass before any C6 approval/interrupt work.** If A1 shows
Eino interrupt/resume cannot meet the Vivy Run state machine, C6 switches to
the outer-loop fallback (RK-3 stop-loss) and this board is re-planned.

> **GATE CLEARED (2026-08-07):** A1 verdict is **GO** — checkpoint-bridge
> approach verified against online Eino v0.9.13 (`docs/eino-capability-verify.md`,
> commit `fe81075`). C6 may proceed once its remaining predecessors (B4, C3)
> land; no outer-loop fallback needed.


## 0.1 Open remaining (updated 2026-09-26)

Completed rows have been moved to `docs/COMPLETE.MD`; deferred, superseded, and declined rows to `docs/DEFER.MD`.
This section retains only currently open work.

Owner-priority consolidation (2026-09-26): Issue #39 is the sole open item. All other unfinished rows were moved to docs/DEFER.MD without being marked complete; each retains its prior status and a reactivation condition.

| ID | Item | Status | Notes |
|---|---|---|---|
| ISSUE-39-EINO-ORCHESTRATION | Native child lifecycle and bounded Eino DAG | OPEN · SOLE PRIORITY · G0 BLOCKED | ORCH-02–07 provisional implementation is present: governed child activation, durable direct mailbox, immutable bounded DAG, native Eino Workflow, model-facing tools, RPC and UI. Focused Go package tests, `go vet`, app builds, UI typecheck and Vitest (402 tests) pass; SQLite conformance ran. G0/G1/G4 remain blocked: `just ci`, PostgreSQL DSN-backed conformance, the integrated R1-R14 matrix and real host/browser E2E are not complete; the owner will run E2E after implementation. No product capability is accepted or released. Current evidence: [provisional verification](logs/2026-09-26-issue39-orch02-provisional/verification.md), [ORCH-01 acceptance](logs/2026-09-26-issue39-orchestration-g0/acceptance.md), [implementation plan](plans/issue39-native-orchestration.md), and [historical Task 5 evidence](logs/2026-09-26-issue39-native-proof/task-5-summary.md). |
Weixin iLink, OneBot (external NapCat), Discord voice, and public webhooks
are **not** on this board; they need their own capability proposal.

> The completed HITL P0 stage is archived in `docs/COMPLETE.MD`; unfinished P1 follow-ups are now
> owner-deferred in `docs/DEFER.MD` until explicitly re-prioritized.
