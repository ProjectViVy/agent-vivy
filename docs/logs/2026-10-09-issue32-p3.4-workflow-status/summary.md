# P3.4 backend slice: truthful workflow run status

Workflow run summaries now expose native Run `status` and engine `engine_status` separately. SQLite and PostgreSQL read the engine projection through one `LEFT JOIN`, fall back to `admitted` when an active run has no projection, and use native status for projection-less terminal runs. The `inofy.listRuns` RPC preserves both fields.

The list and detail paths are covered for completed/succeeded and active/recovery_required states. P3.4 pagination and lifecycle UI presentation remain open under the `oil-frontend` implementation gate.
