# Decisions

Bounded synthetic HTTP server modes create real failed/cancelled native outcomes; no run statuses are seeded. Cancellation waits for the recorded actual provider request and uses public run/cancel.

Existing five-second automatic clock remains intact. Bounded 6/11-second observations test scheduling behavior rather than fabricate timer ticks. Busy/concurrent admission remains separately unproven by these App tests.

The SQLite helper is test observation only; escaping prevents the observer from reading a shortened path or URI-selected alias. Existing production literal-path fix is retained. Windows avoids invalid question-mark filenames while keeping legal fragment/percent coverage; Linux proof is not native Windows acceptance.
