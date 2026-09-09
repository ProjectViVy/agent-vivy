# Chat message action-bar port summary

Date: 2026-08-26
Status: complete

## Outcome

Ported Agent-DIVA’s chat message action bar to Vivy based on
`agent-diva-gui`’s `ChatView.vue` `msg-actions`. Based on user feedback, it was
narrowed to: **user messages (blue bubbles) keep only the timestamp + Edit
(disabled placeholder)**; assistant messages have timestamp + Copy (enabled,
with “Copied” feedback) + Regenerate (enabled) + Rewind / Fork (disabled placeholders).

## Delivered

- `ui/src/lib/chat-actions.ts` — `regeneratePrompt` pure function: the content of
  the nearest user message before the target assistant message;
  `chat-actions.test.ts` has 6 unit tests (nearest selection, skipping
  tool/system, no preceding user, non-assistant target, missing ID, reused ID).
- `ui/src/components/chat/MessageBubble.tsx` — action-bar UI:
  - User messages: Copy + Edit buttons below the bubble (disabled placeholder,
    with “Planned” in the title), revealed on hover by
    `opacity-0 group-hover:opacity-100` (ChatGPT-inspired), with no timestamp;
  - Assistant messages: timestamp (`dateTimeLocale()` localized HH:mm) + Copy
    (Clipboard API first; restricted webviews fall back to a hidden textarea +
    `execCommand('copy')`, then the button changes to “Copied” for 1.5 seconds)
    and Regenerate (disabled when `actionsDisabled` (running/preflight) or no
    preceding user message) + Rewind / Fork (`disabled` placeholders, matching
    Agent-DIVA);
  - Streaming bubbles do not render the action bar; it appears after persistence.
- `ui/src/components/chat/ChatView.tsx` — wiring: `regenerate` uses
  `regeneratePrompt` to obtain the input and reuses the existing `submit`
  (preflight → startRun), without bypassing the preflight product contract.
- `ui/src/i18n/zh.ts` / `en.ts` — `chat.copy / copied / edit / regenerate /
  rewind / fork / pending` entries (same structure).
- `ui/e2e/runtime.spec.ts` — after the first reply, asserts action-bar presence and
  disabled states (user messages keep only Edit, with no Copy/Rewind/Fork), then
  grants clipboard access, clicks Copy, and asserts “Copied” plus the clipboard read-back.

## User-feedback convergence (same day)

The first version put all five actions on both user and assistant messages. After
three rounds of user feedback, it was narrowed (user interaction modeled on ChatGPT):

1. Blue user bubbles keep only Edit + Copy; Rewind / Fork are removed from user
   messages;
2. Remove the timestamp;
3. Place buttons **below the bubble**, hidden with `opacity-0` by default and
   revealed when hovering the message row (reserve the layout space to avoid
   hover jumps), matching ChatGPT behavior.

Assistant messages keep the full action bar (timestamp + Copy + Regenerate +
Rewind / Fork placeholders, `opacity-60` → fully visible on hover).

## Semantic mapping (difference from Agent-DIVA)

Agent-DIVA’s Regenerate **truncates** history after the target assistant message
and overwrites in place. Vivy’s Journal is an append-only source of facts, and
the control plane has no message-truncation / branch RPC, so Regenerate maps to
**starting another conversation turn** from the nearest user input before the
target assistant message (the old answer remains and the new answer is appended).
This is the most honest mapping under the current RPC surface; true Rewind / Fork
and in-place editing require kernel Journal capabilities and are recorded in
`docs/TODO.md` §0.1 (UI-CHAT-ACT).

Agent-DIVA’s streaming retry / stalled badges (`chat.retrying` /
`chat.stalled`, dependent on the `msg.retryStatus` field) were not ported: Vivy’s
run-event contract does not expose that field; connect them when the kernel provides it.

## Explicitly not done

- No backend RPC was added; Journal semantics were not touched.
- Message Edit, Rewind, and Fork were not implemented (disabled placeholders, as in Agent-DIVA).
- Artifact-reference copying (`copyArtifactReference`) was not ported—Vivy tool
  message rendering has no artifact-reference structure.
