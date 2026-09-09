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

1. 390×844: navigation opens; `document.documentElement.scrollWidth <= window.innerWidth`.
2. 1280×720: desktop rail restored (navigation collapsed); send a mock chat; reload keeps the reply.
3. Review center sheet at 390: Approval Details, Approve, and no page-level horizontal scroll.
4. Notebook: open a report, shrink to 390, the heading stays, and Back to List returns to the report list.
5. Memory at 390: open Answer Preferences, use Back to List, and return to the search box.
6. Settings → lifecycle at 390: Promotions tab visible; no horizontal page overflow.

Not verified in a live browser on `http://127.0.0.1:3015` (no browser tools in this session).
