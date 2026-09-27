# PG-5 session replay and activation

The session work subscription now attaches its live buffer, captures a durable WorkSeq watermark, replays through that version, and then delivers later committed events. Notifications include the process epoch and work version. The UI deduplicates session work sequences, refreshes WorkView after reconnect, and opens the newly admitted automatic run through the existing run subscription. A session switch cannot apply an older WorkView response.

The backend remains the authority for activation. The UI does not persist armed state. This slice adds no Plan/Goal controls, second transport, store, queue, scheduler, or Eino implementation. PG-5 Task 2 owns the controls and integrated browser acceptance.

Review follow-up closes three UI races: the first subscription checks its process epoch against the displayed WorkView, initialization cannot reopen an older run after a work event, and a failed event refresh clears activation and retries through the existing subscription reconnect path.
