# UI-INIT-RACE Acceptance Guide

## User-perspective verification steps

### Scenario 1: Create a session and send text immediately during initial page load
1. Open the application (`http://127.0.0.1:3015`).
2. Immediately click the “New session” button in the sidebar as soon as the page starts, before data loading has fully completed.
3. Immediately type in the input box and press Enter to submit the message.
4. **Expected behavior**:
   - The interface remains in the session the user just created and never jumps back to the old or default session.
   - The session drawer shows both the newly created session and the existing historical sessions.
   - The message enters the send flow normally, is not silently swallowed, and the user's message appears in the DOM.

### Scenario 2: Session-mismatch protection and draft retention
1. Simulate triggering message sending when a particular component or network latency causes the session ID values to differ.
2. **Expected behavior**:
   - The content entered in the input box and pending attachments remain intact; the box is not cleared to a blank state.
   - A `RecoverableError` alert bar appears above the chat interface ("The current session does not match the send target; the draft was kept. Please try again.").
   - The user can click Send again or edit the draft and resend it.
