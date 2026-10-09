# P3.3: workflow list cursors

Workflow run pages now use a versioned opaque cursor containing `(created_at, run_id)` and continue under the same descending-timestamp/ascending-ID order in SQLite and PostgreSQL. Definition cursors split on the final colon, preserving existing colon-containing workflow IDs. Invalid and legacy cursors preserve a typed storage error and produce a refreshable RPC invalid-input response.

No UI Module source changed. P3.3 covers backend storage, runtime and RPC cursor contracts; list-navigation UI remains in P3.4.
