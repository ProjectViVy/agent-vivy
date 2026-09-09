# Acceptance: UI audit cleanup

How a human can tell it worked:

1. The Dashboard (`/dashboard`) shows only two tabs — Token and Trajectory. The
   former "Session" tab with the fake 12/2/1 numbers and the recent-activity
   list is gone.
2. The Skills page (`/skills`) has no "Change requests" tab — only "Installed
   skills" and (when enabled) "Market".
3. In the chat input card, the upper toolbar's right side has only History and
   Review Center; the Plus button next to the send button now creates a new
   session (click it: a fresh empty conversation opens). The old do-nothing
   "More" Plus is gone, so there is exactly one Plus in the input card.
4. Chat still works end to end; no amber preflight banner (previous iteration).
