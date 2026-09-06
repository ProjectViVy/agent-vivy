# 2026-09-07 TUI empty assistant line at turn start (audit follow-up)

## Problem

Human audit: after sending the first message, an empty purple assistant
line (just the streaming cursor) popped up immediately, before any model
output existed.

`applyTurnStarted` created an empty streaming assistant draft the moment a
run started. The stream reducer already opens reasoning and answer bubbles
lazily on the first real content, so the draft only ever painted an empty
bubble line; working-state feedback stays with the chrome status line.

## What changed

- `sdk/tui/live/controller.go` — `applyTurnStarted` no longer creates the
  assistant draft; the unused `ensureAssistantDraftLocked` helper was
  removed. The reducer's lazy `EnsureAssistantDraft` (reasoning delta /
  answer delta / completion paths) is the only bubble opener.
- Test: `TestTurnStartCreatesNoEmptyAssistantDraft` — after a turn starts,
  the active session must hold no assistant message without content.

## Explicitly not done

- No renderer change: the streaming cursor (`▌`) still marks bubbles with
  real content while they stream.
- Not pushed (requires explicit authorization).
