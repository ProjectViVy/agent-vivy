# Acceptance

Subscribe to a session after WorkSeq 0. A commit after the subscription's durable watermark but before streaming starts is delivered once after the replayed event. A duplicate event for the same session sequence does not trigger a second run opening.

Restart the control process while the client previously displayed an armed Goal. Reconnection clears that stale projection, reads the backend WorkView and process epoch, and displays the disarmed activation from the restarted process. Switching sessions before a WorkView response completes leaves the newly selected session's view intact.

These behaviors are covered by focused RPC and UI tests. Browser acceptance of the assembled PG-5 controls remains with the integrated Task 2 delivery.
