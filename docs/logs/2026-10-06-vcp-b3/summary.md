# VCP-B3 — GUI steering & follow-up send paths (pi parity)

## Goal

Web face exposes the B1 dual-track queue the same way the TUI does: busy
Enter steers, a secondary chord sends follow-up, queued turns are visible and
individually cancellable, and dequeued text returns to the composer.

## Changes

### Kernel (gap fixes surfaced by GUI needs)

- `internal/runtime/queue.go`:
  - `takeSteerTurn` now persists the steered text as a user `Message` via
    `deps.Messages.AppendMessage`. pi semantics: a consumed steer is a
    permanent transcript row; `HistoryModifier` only edits the resumed model
    history, not the ledger.
  - New `Service.QueueRemove`: per-item cancel across both lanes, journals
    `turn.dequeued{reason:"dequeued", text}` so faces can restore the draft.
- `internal/rpc/control.go`: `queue/remove` verb returning
  `{removed, queue_id, track, text}`.

### Face contract

- `sdk/ui/src/module.ts`: `FaceQueuedTurn`, `FaceQueueState`,
  `FaceQueueTurnResult`, `FaceQueueDequeueResult`, `FaceQueueRemoveResult`,
  `FaceQueueClearResult`; `FaceClientAPI` gained `steerTurn`, `followUpTurn`,
  `getQueueState`, `clearSessionQueue`, `dequeueQueuedTurn`,
  `removeQueuedTurn`; `FaceStoreState` gained `kernelQueue` +
  `queueRestoreText`; store contract gained `steerMessage`,
  `followUpMessage`, `refreshQueue`, `removeKernelQueued`,
  `dequeueQueuedTurn`. Compat tests (`ui-sdk-face-compat`,
  `ui-build-provenance`) kept green — `ui/src/lib/api.ts` aliases the Face
  queue types so both sides share one shape.

### Web face

- `ui/src/lib/api.ts`: queue wire types (aliased to `@vivy/ui-sdk`) + RPC
  wrappers; `getSession` result gained `queue`; `getQueueState` normalizes
  Go `null` slices to `[]`.
- `ui/src/lib/store.ts`:
  - `steerMessage` / `followUpMessage`: text-only submissions ride the
    kernel queue (`kernelEligible`); attachments / continuity references
    fall back to the local FIFO. Kernel steer gets an optimistic user
    bubble; `{queued:false, run_id}` fallbacks adopt the run + subscribe.
  - `admitSettledRun`: post-terminal `queue/state{after_run_id}` poll (same
    mechanism as the TUI's admitSettledCmd) — a kernel-auto-admitted
    follow-up run is adopted as `currentRun` and subscribed, preventing two
    concurrent runs. Early-out when the follow-up lane is empty so an idle
    queue pays no polling delay.
  - `turn.dequeued` events with reason `aborted|cleared` set
    `queueRestoreText{seq}` → ChatView `draftPreset` → composer.
  - Local FIFO still drains only on `run.completed` (Crush parity);
    `failed|cancelled` keep the queue for the user.
- `ui/src/components/chat/ChatInput.tsx`: busy Enter → steer,
  `Shift+Alt+Enter` → follow-up (browser-safe; Alt+Enter is
  browser-reserved), `Alt+Up` → dequeue-restore, Esc → clear queue then
  cancel. Queue pill merges kernel lanes (lane badges) + local FIFO with a
  per-item × (`queue/remove`) and `Clear queue`.
- `ui/src/components/chat/ChatView.tsx`: wires `steerMessage` /
  `followUpMessage` / `dequeueQueuedTurn`; applies `queueRestoreText` once
  per seq.
- i18n `chatInput.steer`, `sendAsFollowUp`, `queueLaneSteer`,
  `queueLaneFollowUp` in en + zh.

## Acceptance evidence

Live browser smoke at `http://127.0.0.1:3015` against a marker-routed mock
LLM (hanging `VIVY-HANG` turn holds the run active):

- Busy Enter → `turn.queued{track:"steer"}` journaled + optimistic bubble +
  `steer` pill chip.
- `Shift+Alt+Enter` → `turn.queued{track:"follow_up"}` + `queued` pill chip.
- `Alt+Up` → `turn.dequeued{reason:"dequeued"}` + text back in composer.
- Pill × → `queue/remove` → `turn.dequeued{reason:"dequeued"}` for the
  steer item.
- `Clear queue` → `turn.dequeued{reason:"cleared"}` + draft restore.
- Cancel → `run.cancelled`; next normal send completed (`MOCK-REPLY`).

Removed steer text disappearing from the transcript after reload is correct:
the optimistic bubble is local-only until the steer is consumed; the
journal stays the single truth.

## Tests

- `internal/runtime` / `internal/rpc`: green (includes
  `TestDequeuePopsNewestFollowUp` from B2 and queue/remove coverage).
- UI: `store.test.ts` +7 (steer/follow-up routing, optimistic bubble,
  fallback-run adoption, kernel-ineligible routing, admitted-run adoption,
  dequeue restore, dequeued-event draft preset); new
  `ChatInput.queue.test.tsx` +5 (key chords, lanes pill, per-item remove,
  idle send path). Full suite: 73 files / 580 tests green.
- Conformance: internal digest re-pinned
  (`a78057a2…` → `b4c238ec…`, 5 sites).

## Notes

- `CancelAfterChatModel` waits for the in-flight model call to return;
  against a hanging model the steer stays `turn.queued` until the call
  ends — identical to TUI behavior, verified here only as queue-state
  truth.
