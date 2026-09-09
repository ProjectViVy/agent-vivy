# Dashboard Token Stats — Real Data

Date: 2026-08-29
Status: complete

## What changed

The dashboard Token Stats tab now reads real `model.usage` events from the
Journal instead of `getDemoTokenUsage` fake data. Closes `UI-TOKEN` in
`docs/TODO.md` §0.1.

### Backend

- **New storage interface** `TokenUsageStore.ListModelUsage(since)` on both
  SQLite and Postgres backends. Queries `run_events` for `type = 'model.usage'`,
  joins `runs` + `sessions`, and attaches the matching `run.started` payload
  (model + provider). Missing `run.started` yields empty strings; the row still
  counts toward totals.
- **Migration 014** (SQLite) adds `run_events_type_created_idx` on
  `(type, created_at)`. Postgres schema updated to include the same index.
- **Pure aggregation** in `internal/rpc/tokenstats.go`: period → since window,
  model distribution, provider grouping, timeline buckets (30min / 1h / 1d),
  session list with primary model. No I/O, fully unit-testable.
- **RPC method** `stats/tokens` registered in `control.go` with capability
  `stats.tokens`. Params: `period`, `tz_offset_minutes`, optional
  `session_limit`. Invalid period → `-32602`.
- **Wiring**: `ControlDeps.TokenUsage` field; `app.go` passes backend.

### Frontend

- **`api.ts`**: new `getTokenUsage()` method + `TokenUsageSnapshot` wire types.
  `stats/tokens` added to `RPC_METHODS`.
- **`format.ts`**: shared `formatTokenCount` extracted from demo-api.
- **`TokenStatsPanel`**: rewritten to call `getTokenUsage` with real params.
  Period switching preserves stale snapshot until new response arrives
  (scope-and-state-integrity contract). Empty state shown when
  `request_count === 0`. Error banner overlays stale data.
- **KPIs**: Total Tokens / Input / Output / Request Count (replaced fake
  "Estimated Cost").
- **Detail view**: Reasoning Tokens + Provider distribution (replaced fake
  Cache Creation/Read + Endpoints). Session table drops cost column.
- **Dashboard route**: `DemoBanner` removed from `_layout.dashboard.tsx`.
  Overview and Trajectory tabs remain demo-marked internally.
- **i18n**: new keys `requestCount`, `reasoningTokens`, `totalReasoning`,
  `providers`, `emptyState` in zh/en.

## Unchanged

- Overview KPI cards and Recent Activity remain demo data (`getDemoDashboard`).
- Trajectory panel remains demo data (`UI-TRAJ` still open).
- No pricing table introduced; cost fields removed rather than showing 0.
- No cache token fields; schema has no cache data.
- `model.usage` event schema unchanged.
- `budget.go` untouched (events/model_calls budget, not token accounting).

## Scope

Backend storage + RPC + UI wiring. No kernel event changes, no schema
migration beyond the index, no pricing or cache work.
