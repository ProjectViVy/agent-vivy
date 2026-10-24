# Steer survives approvals + direct send ("send now")

Date: 2026-10-24
Scope: kernel queue (`internal/runtime`), turn RPC (`internal/rpc`), domain turn
type, UI composer, `@vivy/ui-sdk` Face contract, TUI decode, parity spec.

## Reported defects

1. **Steer lost on approval** — with a run active, adding guidance and then
   hitting a tool approval cleared the guidance. It never reached the model.
2. **No direct-send mode for guidance** — while a run is active the composer
   only offered steer / follow-up. There was no way to interrupt the run and
   start a new one with the typed message.

## Root cause (defect 1)

A steer enqueued while a model call was in flight armed
`adk.WithAgentCancelMode(adk.CancelAfterChatModel)`. When that call returned an
approval-gated tool call, Eino v0.9.13 absorbed the human interrupt into
`*adk.CancelError` (with `InterruptContexts`). `Service.consume` treated it as
the steer's boundary: `takeSteerTurn` journaled `turn.steered`, moved the item to
`q.inFlight`, and resumed. The approval re-registered, but the later resume
bound no steer text and `emitTerminal` flushed `inFlight`, so the guidance
vanished.

## What changed

### Design A — steer survives human interrupts

- `internal/runtime/mapper.go` — the `CancelError` branch now populates
  `m.interrupt` from `CancelError.InterruptContexts` and clears stale interrupt
  state when there are none. `extractInterrupt` scans **all** root-cause
  contexts and classifies the interrupt source from the root-cause payloads
  only (tool approval, plan review, `ask_user`); it never infers "human" from
  `openCalls`. `interruptDetails` gained `Human`.
- `internal/runtime/tooladapter.go` — `ask_user` interrupts now carry the
  `user answer required for <tool>` prefix and `decodeAskUserInterrupt` reads it.
- `internal/runtime/service.go` — a human interrupt absorbed by the pending
  cancel routes to the existing `handleInterrupt` path (durable approval row,
  `s.pending` preserved) instead of `steerPendingCancel`, and disarms the steer.
  Steer peek/resume is guarded by `ctx.Err() == nil`, so a user cancel never
  resurrects a delivery.
- `internal/runtime/queue.go` + `steer_middleware.go` — **delivery on
  injection**: the run peeks the steer head and binds `withSteerText` plus a
  `steerDelivery` token; `turn.steered` is journaled and the item moves to
  `inFlight` only when the middleware actually injects it.
- `internal/runtime/queue.go`, `resumeRun` — an approval resume re-binds the
  same steer text and delivery token, so the guidance lands on the first model
  call after the approval.

A steer typed **while** the run is suspended on an approval still demotes to
follow-up (unchanged).

### Design B — durable "send now"

- `internal/domain/turn_queue.go` / `internal/runtime/payloads.go` — `QueuedTurn`
  and `payloadTurnQueued` carry `interrupt_current` (journal-derived; no
  migration).
- `internal/runtime/queue.go` — `SendNow` / `SendNowWithOptions` enqueue a
  flagged follow-up, then cancel the session's **current** active run; idle
  sessions fall back to a normal start. Flagged items are prepended to the
  follow-up lane. `rebuildQueue` preserves flagged items on `run.cancelled`, so
  replay after a crash still delivers them.
- `internal/runtime/service.go`, `emitTerminal` — the cancelled branch keeps
  flagged follow-ups and flushes everything else (`q.steer`, unflagged
  follow-ups, `q.inFlight`) with `turn.dequeued` reason `aborted`, then drains
  the surviving send-now item.
- `internal/runtime/queue_request.go` — admission clears `interrupt_current` on
  survivors, so a queued item cannot re-interrupt a future run.
- `internal/rpc/control.go` — `turn/send_now` (full turn params: text, mode,
  thinking, face, attachments, references), idle fallback via the existing
  start path.
- UI/SDK: `api.sendNowTurn`, `store.sendNowMessage`, `FaceClientAPI.sendNowTurn`,
  `FaceQueuedTurn.interrupt_current`, `FaceStoreState.sendNowMessage`,
  composer Zap "Send now" button plus Cmd/Ctrl+Enter (intercepted before the
  plain-Enter steer branch), i18n keys, TUI decode field.

## Explicitly not done

- Send-now with attachments or references: those submissions are not
  kernel-eligible, so the composer falls back to the local FIFO (sent after the
  current run settles) instead of interrupting — same fallback steer/follow-up
  already use.
- TUI send-now affordance (the field round-trips; no key binding or command).
- No startup drain of a durable idle queue (pre-existing, tracked separately).
- No `turn.dequeued` events for flush paths outside the cancelled terminal
  (tracked separately).
- No plan/approval UX changes: the approval card, review center, and work bar
  are untouched.

## Delivered files

- Backend: `internal/domain/turn_queue.go`, `internal/runtime/{mapper,tooladapter,
  steer_middleware,queue,queue_request,payloads,service}.go`,
  `internal/runtime/queue_steer_sendnow_test.go`, `internal/rpc/control.go`.
- UI/SDK: `ui/src/lib/{api,store}.ts`, `ui/src/components/chat/{ChatInput,
  ChatView}.tsx`, `ui/src/i18n/{en,zh}.ts`, `sdk/ui/src/module.ts`,
  `sdk/tui/live/rpc.go`.
- Tests: `internal/runtime/queue_steer_sendnow_test.go`,
  `ui/src/components/chat/ChatInput.queue.test.tsx`, `ui/src/lib/store.test.ts`,
  `sdk/ui/src/module.test.ts`.
- Docs: `docs/superpowers/specs/2026-10-06-vivy-code-parity-design.md`,
  `docs/TODO.md`.
