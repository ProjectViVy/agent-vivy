# Trajectory v2 live projection (OBS-04)

`trajectory/session` now emits `projection_version: 2` per the D4 contract.

- Request rows carry stable `request_id` (`run_id:call_id`, legacy
  `run_id:request_start_seq`), `run_id`, `call_id`,
  `call_status` (`active|completed|failed|cancelled|interrupted|legacy`),
  `finished_at`, `usage_state` (`missing|reported|partial|active|legacy`)
  and nullable `usage_evidence`. Legacy `status`/`completed_at`/`usage`
  are preserved.
- Records derive stable IDs from persisted `(run_id, seq, record_kind)`
  instead of positional `rec-N`.
- `run_activity[]` reports per-run `activity_state`
  (`queued|active|waiting|completed|failed|cancelled`) with
  `wait_kind` (`approval|question|child|workflow`) plus
  parent/child/workflow references. Waiting lives on the run, never on a
  finished call; a `model.call.finished` does not terminalize the run;
  open calls on a terminal run become `interrupted`.
- `watermarks` (max folded seq per run) and `has_older_runs` describe the
  recent-run window (default 20, max 50). The RPC advertises
  `trajectory` + `trajectory.v2` capabilities.
- UI: wire/display types extended, run-activity chip strip, call-status /
  usage-state / evidence in the request detail, `TrajectoryLiveTracker`
  performs listener-before-snapshot merge off the single `store.runEvents`
  subscription — (run_id, seq) dedup, contiguous advance, gap/new-run →
  debounced snapshot refetch (the refetch is the reconcile).
