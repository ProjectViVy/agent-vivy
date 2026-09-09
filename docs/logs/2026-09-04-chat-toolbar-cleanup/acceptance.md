# Acceptance — Chat Input Toolbar Fake-Action Cleanup and Closure

## Manual acceptance guide (Product/User View)

1. **Start the environment**:
   - Run `just dev`, or run `just run` and `cd ui; pnpm dev` in two separate terminals.
   - Open `http://127.0.0.1:3015` in a browser.

2. **Inspect the input top bar**:
   - Enter any conversation or click “New session”.
   - Inspect the chat input top bar:
     - **First item on the left**: execution-mode selector dropdown.
     - **Second item on the left**: attachment (paperclip icon).
     - **Center**: the formerly permanent Git branch icon (AutoDream) has **completely disappeared**; the interface has no fake button that is unavailable, unresponsive, or shows an error when clicked.
     - **Permission selector**: the three-way Cautious / Smart / Trust switch remains fully functional.
     - **Right**: the history clock icon and approval-center shield icon display normally.

3. **Verify execution-mode selector interaction**:
   - Click the execution-mode selector (it displays “Agent mode” by default).
   - Expand the dropdown:
     - It shows only “Agent mode” (execute tasks directly) and “Plan mode” (plan first, then execute).
     - It no longer shows “Question mode”, which previously displayed “question mode is not yet implemented” when clicked.
   - Switch to “Plan mode”; the button changes to the Plan mode icon and text. Press Enter to send the input, and the task is submitted as `RunModePlan`.
