# Acceptance

1. Start `vivy-code` in a terminal wide enough to show the right rail.
2. Complete a turn. The active session's message/token/compaction context is
   refreshed without reopening or switching the session.
3. Queue another prompt while a turn is active. The rail shows the real active
   run and queue count, then clears/updates as the terminal event is handled.
4. Change the permission preset. The active-session row in the right rail
   changes with the successful server response.
5. Switch sessions while an older context request is still in flight. A late
   context or permission response must not replace/mutate the newly selected
   session.
6. Press `Ctrl+S` to browse all sessions. The collection appears in the picker,
   not as a repeated list in the right rail.
7. Cancel a run or let it fail while another turn is queued. The follow-up stays
   queued; it is not silently submitted after the unsuccessful terminal state.
