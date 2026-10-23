# Manual acceptance

The owner will perform manual and live-provider acceptance after local
integration. Automated mock smoke evidence is recorded separately in
`verification.md`; it does not claim live-provider acceptance.

1. On local `stable`, start the split backend and Vite UI with a disposable
   user home; open `http://127.0.0.1:3015`.
2. Configure a custom provider whose default model is absent from the entered
   list. Confirm the default appears and can be selected. Refresh models;
   failed discovery must preserve the previous list and selection.
3. Send a request, queue two distinct follow-ups, then refresh while the first
   request is active. Confirm the original remains live and both queued inputs
   remain visible. After settlement, confirm three separate exchanges in FIFO
   order and an empty queue.
4. Confirm relative attachments resolve against the selected Session workspace
   and SQLite paths containing literal `#`, `%`, or spaces reopen correctly.
5. Review the original batches' acceptance records for credential attribution,
   discovery limits, durable failure paths, and rich queued-input options.

Known limit: startup with no active Runs can skip durable idle queue draining
(`QUEUE-IDLE-RESTART-DRAIN`). Automatic recovery in that crash window is not
accepted by this delivery. Existing orphan-blob cleanup limitations also remain.

No push is authorized. Both repair branches and existing worktrees remain
available for follow-up testing.
