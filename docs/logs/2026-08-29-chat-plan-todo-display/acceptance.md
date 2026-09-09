# Acceptance · Real PLAN / TODO in the chat area

At `http://127.0.0.1:3015` (split Vite, not the embedded UI at `:8787`), confirm:

1. **Empty session**
   When a new session has no todos, there is no progress bar above the input box. Click the top-bar todos button: desktop opens
   the right rail / narrow screens open the Sheet, with the copy `This session has no todos`. There are
   no demo items.

2. **In-progress task**
   After the agent calls `task_create` / `task_update` for this session, the progress bar appears: `Tasks` on the
   left, the current `in_progress` `active_form` or `subject` in the center, and `Completed · In progress · Pending` ("Completed · In
   progress · Pending") on the right (zero segments omitted). Click the progress bar to open the right-side list.

3. **Current / historical sections**
   The right-side `Current` contains only pending and in_progress; `History` contains completed and
   cancelled (cancelled is struck through). After refreshing the page, the list remains (persistent; DSH does not clear it on the
   next turn).

4. **Session isolation**
   After switching sessions, the list changes to that session's todos; a late response from the old session cannot overwrite the
   new session.

5. **Chat remains usable**
   If loading todos fails, chat input is not locked; the panel shows an error + retry.
