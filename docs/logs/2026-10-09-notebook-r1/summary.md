# R1 — Bounded manual report generation

## What shipped

- `internal/reportcontract` grew the full bounded-report vocabulary:
  `Window` (period/id/start/end/as_of/completed), `ReportSettings`,
  `SourceRef`, `FeedbackSnapshot`, `Fact`, `FactBundle`, `Narrative`
  (model|empty|fallback|partial + categorized reason), `ReportResult`,
  `GenerationProvenance`, and new error codes (`cancelled`,
  `destination_deleted`, `storage_unavailable`, `outcome_unknown`,
  `recovery_required`).
- `Service.StartReport` now freezes per-run inputs at admission: settings
  resolution, timezone, target section, and the civil-time window are pinned
  into the run's `reportRunInput` so replayed nodes never re-resolve mutable
  state. `GetReport`/`CancelReport`/`ReadReportSettings` project run state
  plus the committed `report_generations` provenance row.
- Sealed effects (`internal/runtime/report_effects.go`): `collect` bounds
  sessions/messages/facts/feedback (200/400/400/64 caps, 2048-byte fact cap)
  and discloses truncation + uncovered days; weekly/monthly reuse the daily
  series generations with daily-read fallback; `narrate` performs exactly one
  tool-free `ChatModel.Generate` (120s, 2048 tokens) and degrades to
  deterministic `fallback` on timeout/malformed/unknown-source output —
  `empty` bundles skip the model; `validate-render` produces bounded
  deterministic markdown (64 KiB cap) with source citations and disclosure
  blocks; `persist` looks up the committed receipt first, then commits the
  publication transaction (generation + revision + head CAS) in
  `notebook_mutations` op-key order with two bounded retries on series
  conflicts — promoted/candidate/destination_deleted, never fabricating a
  deleted entry.
- `cron_jobs` gained `revision` (migration 039 both dialects) used by
  `UpdateCronJobCAS` for settings CAS. `report_generations` (migration 040)
  is provenance+receipt only (`UNIQUE(scope,run_id)` + series/window unique
  entry index); no scheduler state lives there.
- `vivy.reports.{generate,get,cancel,settings.read}` trusted actions ride
  the same owner-facade seam as notebook: scope/actor/origin are bound
  server-side from authenticated identity (Run-bound → agent on workspace
  scope; direct → human on Home), so forged scope/origin cannot enter on
  the wire. Manual generation ignores the disabled settings row, which is
  created/read through the same Storage path (no second config table).
- `vivy/reports` entered `recipes/default.vivy.yml`; generated Assembly now
  emits `ReportFactory` + the four-action ProviderSet; default generation
  baseline inventory updated.

## Decisions

- Replayed idempotent admissions compare the caller's RequestDigest, not the
  volatile pinned input (as_of varies between identical requests).
- First-publication ordering in Postgres creates `notebook_entries` before
  `notebook_revisions` because Postgres enforces the revision→entry FK
  inside the same transaction (SQLite pragma does not).
