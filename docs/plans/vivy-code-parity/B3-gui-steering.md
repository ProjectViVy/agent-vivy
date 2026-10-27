# B3 — GUI single-primary queue/steer

**Goal:** Web face exposes one primary send button whose busy behavior follows the persisted `busy_enter` preference, plus the queue display.
**Epic:** B. **Requirements:** RQ-STEER, RQ-GUI. **Predecessor:** B1.
**Spec:** VCP-D1 §5.2.

## Scope

**Files:** `ui/src/components/chat/ChatInput.tsx` (single primary button + preference-driven Enter), `ui/src/lib/store.ts` + `api.ts` (turn/steer, turn/follow_up, queue/clear), queue indicator component, settings card for `busy_enter`, sdk/ui Face contract + compat test.

## Tasks

- [x] Send affordance: exactly one primary button — Stop when the run is active and the draft is empty, otherwise Send labelled Queue or Steer from `busy_enter` (default `queue`). Keyboard: plain Enter/primary click uses the preference, Cmd/Ctrl+Enter inverts it; Shift+Enter is a newline; Alt+Up dequeues.
- [x] Queue indicator: pending steer/follow-up counts + per-item remove + clear.
- [x] Queue pill bulk action: `Send queued now` clears both lanes, cancels the active run, and re-sends the collected messages in order after it settles.
- [x] `busy_enter` persisted in settings.yaml, exposed on `settings/get` and `settings/update`, editable in Settings → General.
- [x] sdk/ui Face mirrors new ops + queue state field; compat test updated atomically.
- [x] i18n en/zh strings.
- [x] Vitest + browser smoke at :3015 with a live run.

## Boundary

Visual design stays on the VIVY UI kit; no vendored pi UI. No send-now button or verb in the composer: while a run is active the composer offers queue or steer only. The queue pill may offer one bulk action (`Send queued now`) composed in the face as clear + cancel + ordered re-send; it adds no kernel verb and does not touch the primary button.

## Acceptance

GUI steer lands at a turn boundary identically to TUI; queue state matches `session/context` truth after reload; switching the preference flips the primary button label and Enter behavior and survives reload; `Send queued now` leaves the composer empty and the resent messages appear in their original order.
