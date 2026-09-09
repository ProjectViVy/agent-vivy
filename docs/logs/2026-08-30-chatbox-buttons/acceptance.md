# Acceptance (how a person can confirm it)

Open `http://127.0.0.1:3015` (split dev) or the embedded UI:

1. **The chat box can create a session**: the right side of the chat-input top bar
   (left of History) has a ⊕ button with the hover label "Create new session". Clicking
   it immediately switches to a blank new session; opening "History" or the top-bar
   "Sessions" shows one more item in the list.
2. **The old entry points are gone**:
   - The "New session ⊕" button is no longer at the top of the left sidebar.
   - The "Create new session" dashed button is no longer at the top of the
     "History / Sessions" drawer.
   - The microphone (voice) button is no longer on the bottom row of the chat box.
   - The cat-paw (desktop companion) button is no longer on the chat-box top bar.
3. **Execution modes are real**: choose "Plan mode" from the top-bar "Agent mode"
   dropdown and send a tool-triggering message. A preflight banner appears:
   "Preflight blocked this run … unavailable in plan mode". Switch back to "Agent mode"
   and resend to execute normally. Choosing "Ask mode" displays "Ask mode is not
   connected yet" without changing the current selection.
4. **Approval and permissions are already real chat-box features** (not changed this
   time; rechecked):
   - The top-bar shield button opens "Approval Center" with a pending-count badge.
   - The permission dropdown (Cautious / Smart / Trust) takes effect immediately, is
     locked while a run is active, and requires a second confirmation for "Trust".
5. **The following buttons remain placeholders (known to the user; UI retained)**:
   Attachments, AutoDream, "+ More", and Thinking mode (Auto / On / Off). Clicking
   Attachments, AutoDream, or More displays "Not connected yet".

To see item 3 quickly, in Plan mode enter a tool-calling message such as "Help me
create a task".
