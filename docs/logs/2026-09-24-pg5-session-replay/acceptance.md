# Acceptance

Subscribe to a session after WorkSeq 0. A commit after the subscription's durable watermark but before streaming starts is delivered once after the replayed event. A duplicate event for the same session sequence does not trigger a second run opening.

Restart the control process while the client previously displayed an armed Goal. Reconnection clears that stale projection, reads the backend WorkView and process epoch, and displays the disarmed activation from the restarted process. Switching sessions before a WorkView response completes leaves the newly selected session's view intact.

These behaviors are covered by focused RPC and UI tests. Browser acceptance of the assembled PG-5 controls remains with the integrated Task 2 delivery.

If the process restarts between the first WorkView read and subscription attachment, the first subscription epoch invalidates the old armed view even without a later event. If R2 is admitted while session initialization is still loading background runs, R1 cannot reopen over it. If a work-event refresh fails, no stale armed activation remains visible; the subscription retries and accepts the next backend WorkView.
