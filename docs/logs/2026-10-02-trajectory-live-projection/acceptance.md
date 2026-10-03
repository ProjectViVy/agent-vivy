# Acceptance

- [x] `Service.SessionTrajectory(ctx, sessionID, limit)` signature kept;
      RPC method `trajectory/session({session_id, limit?})` unchanged.
- [x] v2 fields per D4: `request_id`, `run_id`, `call_id`, `call_status`,
      `finished_at`, `usage_state`, `usage_evidence`, `run_activity`,
      `watermarks`, `has_older_runs`, `projection_version: 2`.
- [x] Capability advertisement (`trajectory`, `trajectory.v2`) in
      initialize/capabilities.
- [x] Waiting represented on `run_activity` only; call finish does not
      terminalize the run; open calls on terminal runs are `interrupted`.
- [x] Stable record/request IDs; no positional `rec-N` reuse.
- [x] Live merge uses the existing store subscription owner
      (`store.runEvents`) — no second subscription; dedup, gap →
      snapshot reconcile, session-switch/unmount cleanup.
- [x] No historical cursor/page API; no `trajectory-demo-data.ts` import
      in production wiring (test-only).
- [x] `just ci` manual equivalents pass (fmt-check, ui-ci, i18n, vet,
      scoped go tests, conformance gates).
