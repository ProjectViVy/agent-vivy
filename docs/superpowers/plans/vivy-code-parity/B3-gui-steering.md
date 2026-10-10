# B3 — GUI steer/follow-up split

**Goal:** Web face exposes steer vs follow-up send + queue display.
**Epic:** B. **Requirements:** RQ-STEER, RQ-GUI. **Predecessor:** B1.
**Spec:** VCP-D1 §5.2.

## Scope

**Files:** `ui/src/components/chat/ChatInput.tsx` (send split: modifier or dropdown "send as steer/follow-up"), `ui/src/lib/store.ts` + `api.ts` (turn/steer, turn/follow_up, queue/clear), queue indicator component, sdk/ui Face contract + compat test.

## Tasks

- [ ] Send affordance: primary = steer when run active (matching TUI Enter semantics); secondary control = follow-up. Keyboard: Enter=steer, Shift+Alt+Enter=follow-up (browser-safe).
- [ ] Queue indicator: pending steer/follow-up counts + cancel-each affordance.
- [ ] sdk/ui Face mirrors new ops + queue state field; compat test updated atomically.
- [ ] i18n en/zh strings.
- [ ] Vitest + browser smoke at :3015 with a live run.
- [ ] Commit `feat(ui): steer and follow-up send paths`.

## Boundary

Visual design stays on the VIVY UI kit; no vendored pi UI.

## Acceptance

GUI steer lands at a turn boundary identically to TUI; queue state matches `session/context` truth after reload.
