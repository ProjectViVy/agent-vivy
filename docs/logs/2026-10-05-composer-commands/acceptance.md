# Human acceptance

1. Open a new conversation. There is no empty Plan/Goal strip at the top of the transcript.
2. Type `/`. The menu above the input offers `/plan`, `/goal`, and `/skill`. Filter with text; choose using a click, arrow keys plus Enter, or Tab. Selection creates a removable tag and does not run anything yet.
3. Choose `/plan` and submit it alone to enter planning, or add a planning request and submit. The effective Plan chip appears inside the composer only after backend confirmation. Exit using its close button or `/plan off`.
4. Choose `/goal`, type an objective, and set a round limit (default 3). Submit. A real Goal card appears above the composer with status and admitted rounds. Edit, pause, or clear it there. Use `/goal` without text to resume a paused/disarmed current Goal. Clear a completed/blocked Goal before creating another.
5. When Vivy submits a plan, its exact document and revision/one-time execution decisions appear above the input. Use `/goal` with an objective to approve that pending submission and start a bounded Goal. A changed submission requires a fresh command selection.
6. Choose `/skill`, pick an enabled backend skill, and type the request. Its name appears in the tag. Submission requests that skill through the existing runtime middleware. Missing/disabled skills or command failures preserve the draft and show an error.
7. Remove a tag to return to ordinary messaging. Slash paths and unknown commands stay ordinary text. Shift+Enter and Chinese IME retain normal editing. Switching sessions clears command drafts and does not copy another session's rewind preset.

Existing Todo progress remains conditional and positioned above the composer. Permissions, run cancellation, history references, attachments, and the session mask keep their existing authoritative owners.

Screenshots: `desktop-command-menu.png`, `desktop-goal-dock.png`, `desktop-plan-review.png`, `mobile-skill-tag.png`.
