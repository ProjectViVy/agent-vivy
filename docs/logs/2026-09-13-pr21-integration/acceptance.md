# PR #21 integration acceptance

A human or maintainer can recognize the fix by exercising an approval-needed
headless turn under load:

1. The approval request is journaled before cancellation.
2. The headless face reports that approval cannot be collected and cancels the
   run.
3. The matching approval is durably marked cancelled and disappears from the
   pending review queue.
4. The Journal contains one `run.cancelled` terminal and no Tool Provider side
   effect occurs.

The deterministic regression performs this sequence with the real Service,
Eino checkpoint bridge, SQLite approval store, Journal, and event sink.
