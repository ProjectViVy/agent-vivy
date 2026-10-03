# Acceptance — usage coverage projection (OBS-03)

Story contract from `diva-next/observability/OBS-03.md` (in agent-diva):

- [x] `ListModelUsage` / `ListSessionModelUsage` signatures preserved; UsageRow
      extended additively (call_id, attempt_state, usage/bucket flags).
- [x] Projection folds v3 request / v2 usage / v1 finish journal events; no new
      ledger, no DDL.
- [x] Latest valid sample per attempt wins; invalid samples excluded and mark
      the attempt partial; nil vs zero usage distinguished; active / failed /
      cancelled / interrupted / untracked states projected.
- [x] Attempt selection by request start time, not sample time.
- [x] `stats/tokens` emits `projection_version: 2` + `coverage` on snapshot,
      model rows, session rows; `request_count` = usage reports; `cost_known`
      false on partial/missing/active coverage; unknown buckets never priced
      as free.
- [x] Summary-source calls never priced as main.
- [x] SQLite and PostgreSQL share the fold; Postgres parity gated on
      VIVY_POSTGRES_TEST_DSN.
- [x] UI: scope/coverage labels, "—" for unknown cost and unknown
      reasoning/cached buckets, usage-reports label; refresh coalesced from
      the existing run subscription's authoritative events — no second
      permanent event owner.
- [x] Ordered test list implemented and green (see verification.md).
- [x] `just ci` manual equivalents green on Linux (fmt, vet, ui install /
      typecheck / test / build, i18n checks, headless compile, digest gates).

Not covered here (per plan, deferred to OBS-04): attempt-level rows are not
yet surfaced in the trajectory UI; `stats/tokens` totals stay the existing
aggregation shape.
