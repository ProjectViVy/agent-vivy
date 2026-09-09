# Verification record — 2026-08-27 custom provider registry

## Automated gates

| Command | Result |
|---|---|
| `cd ui; pnpm exec vitest run src/components/settings/custom-providers.test.ts` (new cases first) | ✅ 16 / 16 passed |
| `cd ui; pnpm exec vitest run src/components/settings/saved-models.test.ts src/components/settings/custom-providers.test.ts` | ✅ 34 / 34 passed (including updated label-fallback assertions) |
| `cd ui; pnpm typecheck` | ✅ no errors |
| `cd ui; pnpm test` (full vitest suite) | ✅ 15 files / 102 tests all passed (including 16 cases in the new custom-providers.test.ts, updated saved-models.test.ts; i18n zh/en structure-sync tests passed) |
| `just ci` (repository root, = fmt-check + vet + go test + headless-compile + ui-ci[install/typecheck/test/build]) | ✅ all green, ui build `✓ built in 4.06s` (only the existing chunk>500kB warning, unrelated to this change) |

## Browser smoke test (split pair: `just run` :8787 + `cd ui; pnpm dev` :3015)

**Self-testing skipped; delegated to the user's Studio debug session.** When this delivery
was executed on 2026-08-27, `127.0.0.1:8787` (`vivy-backend`) and `127.0.0.1:3015`
(this repository's `pnpm dev` Vite) were still occupied by the user's Vivy Studio debug
session (pid 22900 / 21516, the same as the previous delivery). Per plan, the user session
was left untouched and dev was not started separately; the user's Studio session was used
for the browser-path checks.

The user was advised to quickly walk through the core flow in Studio using `acceptance.md`:

1. Add a custom provider (display name + Base URL + model list) → a row with a 「Custom」
   marker appears in the left column;
2. Click the row → its model list appears on the right; click a model → a chip appears in
   the quick list and the top bar switches immediately;
3. Rename the display name → all saved bookmarks and top-bar labels update;
4. Delete the provider → bookmarks remain (the label falls back to the Base URL hostname),
   and runtime configuration is unchanged;
5. A duplicate Base URL is blocked (an in-form validation error appears);
6. After refresh, both the registry and quick list persist.

## Conclusion

`just ci` is all green (including i18n structure-sync tests and the UI build), satisfying
`just-ci-is-the-gate`; user-visible behavior was delegated because the ports were occupied
by the user's Studio session, and the `smoke-for-user-visible-change` verification record
is this section of the file plus `acceptance.md`.
