# Acceptance

1. Open `http://127.0.0.1:3015` → Settings → “Sandbox”: the label has no “Preview” badge.
2. Change the default permission to “Cautious” and save; after refreshing, it remains Cautious.
3. Create a new session; the chat-area permission shows “Cautious”. Have the model write a file or execute a command:
   the sandbox rejects it rather than showing “Preview updated”.
4. Switch to “Smart”: writes within the workspace go through the approval center.
5. Switch to “Trusted”: confirmation is required; allowlisted read-only tools no longer prompt for approval; commands
   are no longer blocked by the ordinary allowlist.
6. Changing the default in Settings does not affect already-open sessions; it only affects sessions created afterward.
