# Acceptance

Date: 2026-08-26

## User perspective: how to tell the port succeeded

1. Open `http://127.0.0.1:3015` and enter a conversation with messages:
   - **User messages (blue bubbles)**: normally there are no visible controls
     below the bubble; hovering over the message row reveals small Copy + Edit
     buttons below it (Edit is a disabled placeholder), which hide when the pointer leaves;
   - **Assistant messages**: a timestamp + Copy + Regenerate + Rewind / Fork
     below the bubble (the latter two are disabled placeholders), clearer on hover.
2. **Copy**: click Copy on an assistant message → the button briefly changes to
   “Copied” (check mark), and the original message text can be pasted from the
   system clipboard. It also works in the embedded browser (using the fallback path).
3. **Edit**: the disabled placeholder on a user message shows “Edit (Planned)” on
   hover, matching Agent-DIVA; Rewind / Fork are likewise disabled placeholders on
   assistant messages.
4. **Regenerate**: the assistant-message action is clickable when no run is in
   progress. Clicking it starts another turn from the user question associated
   with that answer (still using preflight), appending the new answer at the end
   of the conversation; all Regenerate buttons are disabled while a run is active.
5. Streaming bubbles do not show the action bar until the answer is persisted.

## Acceptance path

- `just ci` is all green (57 unit tests).
- `just ui-e2e` passes (action-bar presence, disabled states, copy feedback, and
  clipboard read-back).
- The developer browser manually verifies steps 1–4 at
  `http://127.0.0.1:3015` (see verification.md).
