# Add Diva-style Token stats to the dashboard

Date: 2026-08-25
Status: complete

## What changed

The dashboard (中控台, `/dashboard`) now has a **Token 统计** section modeled on
Diva's console `TokenStatsPanel`, using local fake data only.

- Overview: period chips (1天 / 3天 / 1周 / 1月 / 6月 / 1年), totals (总 Token /
  输入 / 输出 / 预估费用), model share table, usage trend, session table, export,
  and a detail view (cache, endpoints, per-session input/output).
- Fake snapshots come from `getDemoTokenUsage(period)` in `ui/src/lib/demo-api.ts`.
  They are computed, not stored under `vivy.demo.*`.
- The previous single KPI card **Token 使用** was removed so the same number is
  not shown twice. Session / active-run / review cards stay.

## Unchanged

- No Go kernel, RPC, Journal, or provider change.
- Audit card, recent activity, and the demo banner are untouched.
- Existing unused `getTokenStats(sessionId)` helper remains; it is a different
  per-session shape and is not this panel.

## Scope

UI only, fake data. Wiring to a real usage ledger is `docs/TODO.md` §0.1
`UI-TOKEN`.
