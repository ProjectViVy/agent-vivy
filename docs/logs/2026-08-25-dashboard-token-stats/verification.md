# Verification

## Commands

| Check | Result |
|---|---|
| `just ci` | pass (2026-08-25, exit 0) |
| Playwright against `http://127.0.0.1:3015/dashboard` | pass: 1天 37.1K → 1周 178.1K, export `token-stats-1w.json`, detail/back, 390px no overflow |

`just ci` covers `fmt-check` + `vet` + `test` + `headless-compile` + `ui-ci`
(`pnpm typecheck`, `pnpm test`, `pnpm build`).

## Static checks

- Token panel reads `getDemoTokenUsage`, not `src/lib/rpc.ts`.
- Period snapshot totals equal the sum of session token counts (unit test).
- Session identity is a title (`欢迎使用 Vivy 演示`, …), not a raw id.

## User-visible smoke

Open `http://127.0.0.1:3015/dashboard` in the split pair:

1. KPI row has 会话 / 活跃运行 / 待处理 Review only.
2. **Token 统计** shows 总 Token / 输入 / 输出 / 预估费用.
3. Switching 1天 → 1周 changes the totals.
4. 导出 downloads `token-stats-<period>.json`.
5. 查看详细统计 shows cache + 端点 + 输入/输出; 返回 restores the overview.
