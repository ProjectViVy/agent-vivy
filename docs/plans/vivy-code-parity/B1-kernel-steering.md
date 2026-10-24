# B1 — Kernel dual-track queue + steering

**Goal:** Kernel-owned steer/follow-up queue with turn-boundary steer injection; RPC `turn/steer`, `turn/follow_up`, `queue/clear`, `queue/mode`; Journal events `turn.queued`, `turn.steered`, `turn.dequeued`.
**Epic:** B. **Requirements:** RQ-STEER.
**Spec:** [VCP-D1](../../specs/2026-10-06-vivy-code-parity-design.md) §5.2. **Baseline:** `f34f3ce`. **Risk:** highest in the program — the injection mechanism is decided by the Eino check below.

## Prerequisites

Read `internal/runtime/service.go` consume loop (`s.consume`, ~3391), the `waitForToolsSettled`/`consumeChildMailboxSafePoint` precedent (~2900–2920), `internal/runtime/mapper.go` turn events, and `sdk/tui/live/controller.go` `queuedTurn` (the local FIFO this replaces).

## Tasks

- [ ] **Eino capability check (mandatory, gate the rest):** inspect pinned eino v0.9.13 `adk` for mid-run user-message injection (Runner/ChatModelAgent/`AgentRunOption`/state middleware). If a native seam exists → mechanism M1 (in-loop inject). Else → mechanism M2 (early-settle + immediate continuation segment at the next turn-boundary safe point, reusing cancel/checkpoint + `turn.steered` as the continuity marker). Record the inspected APIs and the choice in the story log.
- [ ] Domain: `QueuedTurn{ID, SessionID, Track(steer|follow_up), Text, Thinking, Mode, Attachments, ContextPaths, CreatedAt}` + Journal events. Queue persistence rides the Journal (no new table unless a marker pattern is insufficient — prefer the `session_truncations` marker precedent).
- [ ] Service: `Steer`, `FollowUp`, `ClearQueue`, `SetQueueModes`, `QueueState` methods; steer consumption at turn boundary inside `consume`; follow-up auto-admission after terminal settle (`all` drains into one delivery, `one-at-a-time` head only).
- [ ] RPC: `turn/steer`, `turn/follow_up`, `queue/clear`, `queue/mode`, plus queue state in `session/context`/`session/status` payload. 409 on steer when no active run → fall back to follow_up track.
- [ ] Busy rules: steer during no-run = start a run (equivalent to prompt); dequeue returns text to the caller for editor restore.
- [ ] Tests: steer lands at a turn boundary not mid-tool-batch; follow-up runs after settle; clear/dequeue journaled; restart mid-queue replays correctly; mode `one-at-a-time` leaves tail intact; concurrent steer+follow_up ordering deterministic.
- [ ] `go test ./internal/runtime ./internal/rpc -run 'Steer|Queue|FollowUp'` then full `just ci`.
- [ ] Commit `feat(runtime): dual-track message queue with turn-boundary steering`.

## Boundary

No face changes (B2/B3). `s.active`-keyed busy semantics unchanged; steer never interrupts an in-flight tool call (delivery is at boundary only — same as pi). Abort semantics: abort flushes both tracks to dequeue events (pi returns queued messages to the editor; the face handles re-editing).

## Acceptance

Steering text appears in the model-visible history of the same logical interaction without a second user-facing run boundary artifact (or with `turn.steered` as the single continuity marker under M2); queue survives process restart; all four RPC verbs round-trip.
