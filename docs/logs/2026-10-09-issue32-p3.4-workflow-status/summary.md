# P3.4 workflow pagination and truthful run status

Workflow run summaries expose native Run `status` and engine `engine_status` separately. SQLite and PostgreSQL read the engine projection through one `LEFT JOIN`, fall back to `admitted` when an active run has no projection, and use native status for projection-less terminal runs. The `inofy.listRuns` RPC preserves both fields.

The UI now loads subsequent workflow and Run pages, retains loaded rows and the same cursor after a continuation failure, replaces results on refresh, deduplicates rows, and fences late requests after refresh or session changes. Empty wire cursors normalize to `null`. Run views display native and engine status separately, disable cancellation for native terminal states including `completed`, and refresh details when `run_recovery_required` arrives; no Resume action is offered.

The implementation follows the documented `oil-frontend` unavailable-skill ruling: it reuses Module conventions and limits changes to specified interaction/data behaviors without visual redesign. Focused tests, full UI, typecheck and affected Go package checks pass. Real-browser candidate acceptance and final aggregate/conformance gates remain owned by P7.
