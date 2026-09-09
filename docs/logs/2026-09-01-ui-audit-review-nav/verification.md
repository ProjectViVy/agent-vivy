# Verification

| Command | Result |
|---|---|
| `pnpm exec playwright test e2e/runtime.spec.ts` (initial run without rebuilding the bundle) | passed—but the runtime.spec tail assertion was after the `hasRealProvider` early return, so this provider-less environment did not actually execute the new assertion; it was moved to a standalone provider-independent spec |
| `pnpm exec playwright test e2e/approvals-nav.spec.ts` (first direct run without rebuilding the bundle) | failed—the welcome wizard rendered asynchronously after a one-shot `isVisible()` check, so the skip action was skipped and the wizard intercepted the navigation click (60s timeout). Fixed by using a bounded 5s `waitFor` before deciding whether to skip |
| `just ui-e2e` (rebuild embedded bundle with pnpm build + full e2e) | passed—11 passed (including `approvals-nav.spec.ts:6 Approvals reachable directly from main navigation 662ms`), 0 failed |
| `just ci` (fmt-check + vet + go test + headless + plugin-ci + UI typecheck/test/build) | passed (exit 0) |

Key points:
- Putting the navigation assertion in `runtime.spec.ts` was ineffective because
  the provider early return runs first; it was moved to the standalone
  `approvals-nav.spec.ts`, which also runs without a key. The Playwright webServer
  serves the built embedded UI, so `pnpm build` must run first (`just ui-e2e`
  already includes it).
- The welcome wizard appears asynchronously, so a one-shot `isVisible()` check
  races; use a bounded `waitFor` before deciding whether to skip.
