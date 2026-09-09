# Acceptance — UI-CHAT-ACT R2 (edit / rewind / fork)

How to manually confirm this slice is working (development environment `just run` + `cd ui; pnpm dev` →
http://127.0.0.1:3015):

1. **Edit**: send a message in any session (it is recorded even offline; a failed turn is fine). Hover over your own
   blue bubble → "Copy / Edit" appears below the message → click "Edit", and the bubble becomes a
   text box in place (pre-filled with the original text) → edit the text → click the checkmark (save and rerun): the old input disappears from the view,
   and the new input is sent again as a new turn. The original input remains in the database (`session/messages` folded,
   no rows deleted).
2. **Rewind** (when offline and there is no assistant bubble, verify in a session with an assistant turn): hover over an assistant message →
   "Rewind to here" → confirmation dialog ("This message and everything after it will leave the context; the original text is retained and will not be
   deleted.") → after confirmation, the selected message (inclusive) leaves the view, and the input box is automatically pre-filled with
   the most recent user input that remains in context, ready to edit and resend.
3. **Fork**: hover over an assistant message → "Fork from here" → confirmation dialog ("Create a new session using the history through this message;
   the original session remains unchanged.") → after confirmation, automatically jump to the new session; the session list shows
   a child session named "Original title (fork)" or using a custom title, and the child session contains all
   history through and including the fork point; the original session view is unchanged.
4. **Tail-anchor regression** (the core correction in this slice): continue sending new messages after rewind/edit—the new turn must
   appear in the view (the old implementation permanently folded everything after rewind, causing the retry to "evaporate").
5. All three buttons are disabled during a run; failed actions appear in a retryable error bar at the bottom of the chat.

Automation equivalent: `ui/e2e/chat-act.spec.ts` (offline end-to-end flow) and
`ui/e2e/runtime.spec.ts` existing main spec's assertion that all three buttons are enabled.
