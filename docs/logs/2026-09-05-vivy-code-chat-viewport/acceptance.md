# Acceptance

1. Open a session with more history than the terminal height. Confirm the initial view is pinned to the newest message.
2. Press `PgUp` or wheel upward in the chat. Confirm older messages appear and the footer exposes `end latest`.
3. Let new reasoning or assistant text stream while paused. Confirm the visible history does not jump to the bottom.
4. Press `End`, `PgDn` to the boundary, or submit a message. Confirm follow-latest resumes and the newest content is visible.
5. Click the right rail and wheel to scroll it; click outside it and wheel to scroll chat. Confirm neither surface moves while Sessions, command/file completion, or a gate is open.
6. Switch sessions and confirm the new session starts at its latest content without inheriting the previous offset.
7. Resize until all history fits and confirm stale paused state disappears.
8. Enter combining text or emoji and backspace once; confirm the full visible grapheme is removed rather than leaving a broken modifier or joiner.
9. Render long tool output at narrow widths and confirm it wraps inside the viewport without terminal title/cursor manipulation.
