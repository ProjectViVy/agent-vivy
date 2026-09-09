# Verification

## Commands

| Check | Result |
|---|---|
| `just ci` | pass (2026-08-25, exit 0) |
| Playwright against `http://127.0.0.1:3015/dashboard` | pass: 1 day 37.1K → 1 week 178.1K, export `token-stats-1w.json`, detail/back, 390px no overflow |

`just ci` covers `fmt-check` + `vet` + `test` + `headless-compile` + `ui-ci`
(`pnpm typecheck`, `pnpm test`, `pnpm build`).

## Static checks

- Token panel reads `getDemoTokenUsage`, not `src/lib/rpc.ts`.
- Period snapshot totals equal the sum of session token counts (unit test).
- Session identity is a title (`Welcome to the Vivy Demo`, …), not a raw id.

## User-visible smoke

Open `http://127.0.0.1:3015/dashboard` in the split pair:

1. KPI row has Sessions / Active Runs / Pending Reviews only.
2. **Token Stats** shows Total Tokens / Input / Output / Estimated Cost.
3. Switching 1 day → 1 week changes the totals.
4. Export downloads `token-stats-<period>.json`.
5. View Detailed Stats shows cache + endpoints + input/output; Back restores the overview.
