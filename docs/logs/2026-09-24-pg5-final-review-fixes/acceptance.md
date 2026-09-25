# Acceptance

1. Open a session with a Goal. The summary shows its objective, phase, and spent/maximum rounds; input controls appear only after selecting Edit Goal.
2. Edit the objective and round limit, then save. The existing Goal changes revision while its spent rounds remain visible.
3. Cause a stale Goal conflict during save. The draft and opening GoalRef remain in the form, the store error is visible, and a later click does not silently target the newer Goal revision.
4. Observe `goal.round_admitted` on an already-open subscriber and immediately fetch WorkView. Its `current_run_id` is the admitted event's RunID.
