# Verification

## Commands

| Check | Result |
|---|---|
| `pnpm typecheck` (in `ui/`) | pass |
| `pnpm test` (in `ui/`) | pass (30 tests) |
| `just ci` | pass (2026-08-25, exit 0) |
| `just ui-e2e` | pass (1 Playwright test, 12.6s) |

## User-visible smoke

Playwright `ui/e2e/runtime.spec.ts` (embedded UI after `pnpm build`, not Vite `:3015`):

1. 390×844: 打开导航 visible; `document.documentElement.scrollWidth <= window.innerWidth`.
2. 1280×720: desktop rail restored (收起导航); send a mock chat; reload keeps the reply.
3. Review center sheet at 390: 审批详情, 批准, no page-level horizontal scroll.
4. Notebook: open a report, shrink to 390, heading stays, 返回列表 returns to the report list.
5. Memory at 390: open 回答偏好, 返回列表, search box returns.
6. Settings → lifecycle at 390: Promotions tab visible; no horizontal page overflow.

Not verified in a live browser on `http://127.0.0.1:3015` (no browser tools in this session).
