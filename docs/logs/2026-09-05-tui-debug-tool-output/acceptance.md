# Acceptance

1. Start `vivy-code` with the default configuration and have the model call a tool that produces a large result (for example, listing a large directory).
2. A completed tool card shows at most 8 lines of body text, ending with `… N more lines · set tui.debug: true`; the chat page is no longer filled by the entire tool result.
3. Add the following to the configuration in use:

   ```yaml
   tui:
     debug: true
   ```

4. Restart and run the same tool; the tool card shows the complete result without an ellipsis notice.
5. In both modes, the tool execution result, session replay data, and model behavior should remain consistent.
