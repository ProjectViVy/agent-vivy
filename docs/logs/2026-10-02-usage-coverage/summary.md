# Usage coverage projection (OBS-03)

Token stats now explain their evidence instead of silently summing samples.

- New `internal/storage/usage_projection.go` folds the journal (model.request
  v3 / model.usage v2 / model.call.finished v1 / run.started) into per-attempt
  `UsageRow`s keyed by call_id. Latest valid usage sample wins; invalid samples
  mark the attempt partial; orphan v2 samples become untracked attempts;
  pre-lifecycle v1 usage events still emit legacy rows. Selection is by attempt
  start time, not sample time; summary-source calls never inherit the run's
  pricing attribution.
- `stats/tokens` adds `projection_version: 2` and a `coverage` block
  (empty/complete/partial/legacy + per-bucket counts + unknown_buckets) on the
  snapshot, each model row, and each session row. `request_count` stays
  "usage reports" (reported calls + legacy records), never billed requests.
  `cost_known` is false whenever coverage is partial or active/missing — an
  incomplete subtotal is never rendered as the charge.
- TokenStatsPanel renders the coverage line, masks reasoning/cached buckets
  shown as unknown ("—" not 0), and refreshes coalesced from the existing run
  subscription's authoritative events (model.usage / model.call.finished /
  run terminal) — no second subscription owner.

Both SQLite and PostgreSQL readers share the same fold
(`usageProjectionEventTypes`, one joined event+run+session query); the Postgres
parity suite is gated on VIVY_POSTGRES_TEST_DSN.
