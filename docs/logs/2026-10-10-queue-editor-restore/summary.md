# Complete queued turn editor restoration

GUI and TUI Alt+Up recall now retain the complete queued turn while its text is
edited: execution mode, thinking preference, image snapshots, captured file
contexts, face, policy, collaboration settings, and continuity request ID,
selectors, and history scope. Resend forwards captured bytes without reading the
original paths again. The GUI restores image thumbnails, mode/thinking controls,
reference chips and removable captured contexts; the TUI keeps raw bytes private
and renders only image/file metadata through its existing surface.

Abort and clear return each full DTO as a separate editor recovery item. Faces
show explicit recall and never submit these items automatically. Existing user
drafts are protected, including drafts typed while recall is in flight. Failed
sends retain captured payloads and user edits for retry. Unsupported mode,
thinking, or face values prevent durable dequeue. The inspected queue ID is sent
with recall so a newer concurrent turn cannot be consumed without validation.

Source dependencies are queue core dfaca432608359db60788b7669721ca59489e9a4
and conditional dequeue e65ff27406609b8b7304af394cacaebaa3a300e8. These commits
were cherry-picked into the isolated worktree before this independent adapter
patch. The parent owns integration, full product CI and browser verification.
There is no second queue, automatic face scheduler, database, or deployment.
