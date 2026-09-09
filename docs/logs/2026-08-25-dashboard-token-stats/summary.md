# Add Diva-style Token stats to the dashboard

Date: 2026-08-25
Status: complete

## What changed

The dashboard (Dashboard, `/dashboard`) now has a **Token Stats** section modeled on
Diva's console `TokenStatsPanel`, using local fake data only.

- Overview: period chips (1 day / 3 days / 1 week / 1 month / 6 months / 1 year), totals (Total Tokens /
  Input / Output / Estimated Cost), model share table, usage trend, session table, export,
  and a detail view (cache, endpoints, per-session input/output).
- Fake snapshots come from `getDemoTokenUsage(period)` in `ui/src/lib/demo-api.ts`.
  They are computed, not stored under `vivy.demo.*`.
- The previous single KPI card **Token Usage** was removed so the same number is
  not shown twice. Session / active-run / review cards stay.

## Unchanged

- No Go kernel, RPC, Journal, or provider change.
- Audit card, recent activity, and the demo banner are untouched.
- Existing unused `getTokenStats(sessionId)` helper remains; it is a different
  per-session shape and is not this panel.

## Scope

UI only, fake data. Wiring to a real usage ledger is `docs/TODO.md` §0.1
`UI-TOKEN`.
